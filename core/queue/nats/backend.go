package nats

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"uuid"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	corenats "github.com/retailcrm/mg-transport-core/v2/core/nats"
	"github.com/retailcrm/mg-transport-core/v2/core/queue"
)

// ProvisionMode selects how the driver obtains its JetStream stream and consumer.
type ProvisionMode uint8

const (
	// BindExisting binds to an already provisioned stream and consumer and validates their
	// configuration instead of creating anything.
	BindExisting ProvisionMode = iota
	// Ensure creates or updates the stream and the durable consumer, covering the queue and schedule
	// subjects.
	Ensure
)

// PayloadMode selects the bytes stored in JetStream.
type PayloadMode uint8

const (
	// PayloadEnvelope stores envelope metadata and the encoded value in an internal JSON envelope.
	PayloadEnvelope PayloadMode = iota
	// PayloadRaw stores only the codec output and derives envelope metadata from the NATS message.
	PayloadRaw
)

// DeadLetterConfig configures a separate stream used to preserve poison messages. Subject is the
// destination for this driver; Stream must cover it. Provisioning follows the parent Config mode.
type DeadLetterConfig struct {
	Subject string
	Stream  jetstream.StreamConfig
}

// Config configures a JetStream queue driver.
type Config struct {
	// Subject is the subject ready items are published to and the consumer filters on. Required.
	Subject string
	// ScheduleSubject is the prefix for scheduled (deferred) messages; it defaults to Subject +
	// ".schedule". The stream must cover "<ScheduleSubject>.>".
	ScheduleSubject string
	// Stream is the JetStream stream configuration. Ensure uses it verbatim (adding the queue and
	// schedule subjects and enabling message schedules when Subjects is empty); BindExisting uses
	// only the Name for lookup and validates the rest.
	Stream jetstream.StreamConfig
	// Consumer is the durable pull consumer configuration. Either Name or Durable must be set; the
	// other defaults to the provided one. The driver enforces explicit acknowledgments and the
	// queue-subject filter.
	Consumer jetstream.ConsumerConfig
	// Provision selects between creating the resources and binding to existing ones.
	Provision ProvisionMode
	// FetchMaxWait bounds a single consumer fetch while dequeue-polling; it defaults to one second.
	FetchMaxWait time.Duration
	// PayloadMode defaults to PayloadEnvelope. PayloadRaw is compatible with producers that publish
	// codec bytes directly to the queue subject.
	PayloadMode PayloadMode
	// DisableScheduling permits binding to streams without message schedules. Delayed enqueue then
	// returns queue.ErrSchedulingUnsupported; delayed negative acknowledgments remain available.
	DisableScheduling bool
	// DeadLetter optionally preserves malformed and explicitly dead-lettered messages.
	DeadLetter *DeadLetterConfig
}

// Driver is a queue.Driver implementation over one JetStream stream and one durable pull consumer.
// It is safe for concurrent use.
type Driver[T any] struct {
	js       jetstream.JetStream
	stream   jetstream.Stream
	consumer jetstream.Consumer
	codec    queue.Codec[T]
	config   Config
	closed   atomic.Bool
	closeCtx context.Context
	cancel   context.CancelFunc
}

type envelope struct {
	ID         string    `json:"id"`
	EnqueuedAt time.Time `json:"enqueuedAt"`
	Payload    []byte    `json:"payload"`
}

// New builds a JetStream queue driver from a connected core NATS client, an item codec, and a
// configuration. Depending on Config.Provision it creates or updates the stream and consumer (Ensure)
// or binds to and validates existing ones (BindExisting).
func New[T any](
	ctx context.Context,
	client *corenats.Client,
	codec queue.Codec[T],
	config Config,
) (*Driver[T], error) {
	if client == nil || client.JetStream == nil {
		return nil, errors.New("NATS JetStream client is required")
	}
	if codec == nil {
		return nil, errors.New("NATS queue codec is required")
	}
	if config.Subject == "" || config.Stream.Name == "" {
		return nil, errors.New("NATS queue subject and stream name are required")
	}
	if config.ScheduleSubject == "" {
		config.ScheduleSubject = config.Subject + ".schedule"
	}
	if config.Consumer.Name == "" && config.Consumer.Durable == "" {
		return nil, errors.New("NATS durable consumer name is required")
	}
	if config.Consumer.Name == "" {
		config.Consumer.Name = config.Consumer.Durable
	}
	if config.Consumer.Durable == "" {
		config.Consumer.Durable = config.Consumer.Name
	}
	if config.FetchMaxWait <= 0 {
		config.FetchMaxWait = time.Second
	}
	if config.PayloadMode != PayloadEnvelope && config.PayloadMode != PayloadRaw {
		return nil, errors.New("unsupported NATS queue payload mode")
	}
	if config.DeadLetter != nil && (config.DeadLetter.Subject == "" || config.DeadLetter.Stream.Name == "") {
		return nil, errors.New("NATS dead-letter subject and stream name are required")
	}

	closeCtx, cancel := context.WithCancel(context.Background())
	b := &Driver[T]{js: client.JetStream, codec: codec, config: config, closeCtx: closeCtx, cancel: cancel}
	var err error
	if config.Provision == Ensure {
		err = b.ensure(ctx)
	} else {
		err = b.bind(ctx)
	}
	if err != nil {
		cancel()
		return nil, err
	}
	return b, nil
}

func (b *Driver[T]) ensure(ctx context.Context) error {
	config := b.config.Stream
	config.AllowMsgSchedules = !b.config.DisableScheduling
	if len(config.Subjects) == 0 {
		config.Subjects = []string{b.config.Subject}
		if !b.config.DisableScheduling {
			config.Subjects = append(config.Subjects, b.config.ScheduleSubject+".>")
		}
	}
	if err := validateSubjects(config.Subjects, b.config.Subject, b.scheduleSubject()); err != nil {
		return err
	}
	stream, err := b.js.CreateOrUpdateStream(ctx, config)
	if err != nil {
		return fmt.Errorf("ensure NATS stream %q: %w", config.Name, err)
	}
	consumerConfig := b.config.Consumer
	consumerConfig.AckPolicy = jetstream.AckExplicitPolicy
	consumerConfig.FilterSubject = b.config.Subject
	consumer, err := stream.CreateOrUpdateConsumer(ctx, consumerConfig)
	if err != nil {
		return fmt.Errorf("ensure NATS consumer %q: %w", consumerConfig.Name, err)
	}
	b.stream, b.consumer = stream, consumer
	return b.ensureDeadLetter(ctx)
}

func (b *Driver[T]) bind(ctx context.Context) error {
	stream, err := b.js.Stream(ctx, b.config.Stream.Name)
	if err != nil {
		return fmt.Errorf("bind NATS stream %q: %w", b.config.Stream.Name, err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return fmt.Errorf("inspect NATS stream %q: %w", b.config.Stream.Name, err)
	}
	if !b.config.DisableScheduling && !info.Config.AllowMsgSchedules {
		return errors.New("NATS stream does not allow message schedules")
	}
	if err := validateSubjects(info.Config.Subjects, b.config.Subject, b.scheduleSubject()); err != nil {
		return err
	}
	consumer, err := stream.Consumer(ctx, b.config.Consumer.Name)
	if err != nil {
		return fmt.Errorf("bind NATS consumer %q: %w", b.config.Consumer.Name, err)
	}
	consumerInfo, err := consumer.Info(ctx)
	if err != nil {
		return fmt.Errorf("inspect NATS consumer %q: %w", b.config.Consumer.Name, err)
	}
	if consumerInfo.Config.AckPolicy != jetstream.AckExplicitPolicy ||
		consumerInfo.Config.FilterSubject != b.config.Subject {
		return errors.New("NATS consumer must use explicit acknowledgments and the configured queue subject")
	}
	b.stream, b.consumer = stream, consumer
	return b.bindDeadLetter(ctx)
}

func (b *Driver[T]) scheduleSubject() string {
	if b.config.DisableScheduling {
		return ""
	}
	return b.config.ScheduleSubject
}

func (b *Driver[T]) ensureDeadLetter(ctx context.Context) error {
	if b.config.DeadLetter == nil {
		return nil
	}
	config := b.config.DeadLetter.Stream
	if len(config.Subjects) == 0 {
		config.Subjects = []string{b.config.DeadLetter.Subject}
	}
	if !coveredBy(config.Subjects, b.config.DeadLetter.Subject) {
		return fmt.Errorf("NATS dead-letter stream does not cover subject %q", b.config.DeadLetter.Subject)
	}
	_, err := b.js.CreateOrUpdateStream(ctx, config)
	if err != nil {
		return fmt.Errorf("ensure NATS dead-letter stream %q: %w", config.Name, err)
	}
	return nil
}

func (b *Driver[T]) bindDeadLetter(ctx context.Context) error {
	if b.config.DeadLetter == nil {
		return nil
	}
	stream, err := b.js.Stream(ctx, b.config.DeadLetter.Stream.Name)
	if err != nil {
		return fmt.Errorf("bind NATS dead-letter stream %q: %w", b.config.DeadLetter.Stream.Name, err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return fmt.Errorf("inspect NATS dead-letter stream %q: %w", b.config.DeadLetter.Stream.Name, err)
	}
	if !coveredBy(info.Config.Subjects, b.config.DeadLetter.Subject) {
		return fmt.Errorf("NATS dead-letter stream does not cover subject %q", b.config.DeadLetter.Subject)
	}
	return nil
}

// Enqueue publishes the encoded item to the queue subject, or to the schedule subject with a
// schedule-at time when the item is deferred. The enqueue ID is used as the JetStream message ID for
// deduplication.
func (b *Driver[T]) Enqueue(ctx context.Context, value T, options queue.EnqueueOptions) error {
	if b.closed.Load() {
		return context.Canceled
	}
	payload, err := b.codec.Encode(value)
	if err != nil {
		return fmt.Errorf("encode NATS envelope: %w", err)
	}
	now := time.Now()
	id := options.ID
	if id == "" {
		id = uuid.New().String()
	}
	body := payload
	if b.config.PayloadMode == PayloadEnvelope {
		body, err = json.Marshal(envelope{ID: id, EnqueuedAt: now, Payload: payload})
		if err != nil {
			return fmt.Errorf("encode NATS envelope: %w", err)
		}
	}
	message := &natsgo.Msg{
		Subject: b.config.Subject,
		Data:    body,
		Header:  natsgo.Header{jetstream.MsgIDHeader: []string{id}},
	}
	publishOptions := []jetstream.PublishOpt{jetstream.WithMsgID(id)}
	if options.NotBefore.After(now) {
		if b.config.DisableScheduling {
			return queue.ErrSchedulingUnsupported
		}
		scheduleToken := strings.ReplaceAll(uuid.New().String(), "-", "")
		message.Subject = b.config.ScheduleSubject + "." + scheduleToken
		publishOptions = append(
			publishOptions,
			jetstream.WithScheduleAt(options.NotBefore),
			jetstream.WithScheduleTarget(b.config.Subject),
		)
	}
	if _, err := b.js.PublishMsg(ctx, message, publishOptions...); err != nil {
		return fmt.Errorf("publish NATS envelope: %w", err)
	}
	return nil
}

// Dequeue fetches the next message from the durable consumer. Messages whose envelope or payload
// cannot be decoded are terminated; the error is returned to the caller, and the next Dequeue attempt
// fetches the following message.
func (b *Driver[T]) Dequeue(ctx context.Context) (queue.JobEnvelope[T], error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if b.closed.Load() {
			return nil, context.Canceled
		}
		fetchCtx, cancel := context.WithTimeout(ctx, b.config.FetchMaxWait)
		stop := context.AfterFunc(b.closeCtx, cancel)
		message, err := b.consumer.Next(jetstream.FetchContext(fetchCtx))
		stop()
		cancel()
		if ctx.Err() != nil {
			if message != nil {
				_ = message.Nak()
			}
			return nil, ctx.Err()
		}
		if b.closed.Load() {
			if message != nil {
				_ = message.Nak()
			}
			return nil, context.Canceled
		}
		if errors.Is(err, natsgo.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("fetch NATS envelope: %w", err)
		}
		metadata, err := message.Metadata()
		if err != nil {
			_ = message.Nak()
			return nil, fmt.Errorf("read NATS envelope metadata: %w", err)
		}
		body, err := b.decodeMessage(message, metadata)
		if err != nil {
			return nil, b.rejectMalformed(ctx, message, err)
		}
		value, err := b.codec.Decode(body.Payload)
		if err != nil {
			return nil, b.rejectMalformed(ctx, message, fmt.Errorf("decode NATS envelope: %w", err))
		}
		return &jobEnvelope[T]{
			driver:  b,
			message: message,
			value:   value,
			metadata: queue.Metadata{
				ID: body.ID, EnqueuedAt: body.EnqueuedAt,
				DeliveredAt: time.Now(), Attempt: metadata.NumDelivered,
			},
		}, nil
	}
}

func (b *Driver[T]) decodeMessage(message jetstream.Msg, metadata *jetstream.MsgMetadata) (envelope, error) {
	if b.config.PayloadMode == PayloadRaw {
		id := message.Headers().Get(jetstream.MsgIDHeader)
		if id == "" {
			id = strconv.FormatUint(metadata.Sequence.Stream, 10)
		}
		return envelope{ID: id, EnqueuedAt: metadata.Timestamp, Payload: message.Data()}, nil
	}
	var body envelope
	if err := json.Unmarshal(message.Data(), &body); err != nil {
		return envelope{}, fmt.Errorf("decode NATS envelope: %w", err)
	}
	return body, nil
}

func (b *Driver[T]) rejectMalformed(ctx context.Context, message jetstream.Msg, cause error) error {
	if b.config.DeadLetter == nil {
		_ = message.Term()
		return cause
	}
	if err := b.publishDeadLetter(ctx, message, cause); err != nil {
		_ = message.Nak()
		return errors.Join(cause, err)
	}
	if err := message.Term(); err != nil {
		return errors.Join(cause, fmt.Errorf("terminate malformed NATS envelope: %w", err))
	}
	return cause
}

func (b *Driver[T]) publishDeadLetter(ctx context.Context, message jetstream.Msg, cause error) error {
	if b.config.DeadLetter == nil {
		return queue.ErrDeadLetterUnsupported
	}
	header := maps.Clone(message.Headers())
	if header == nil {
		header = make(natsgo.Header)
	}
	header.Set("X-Error", cause.Error())
	header.Set("X-Original-Subject", message.Subject())
	deadLetter := &natsgo.Msg{Subject: b.config.DeadLetter.Subject, Header: header, Data: message.Data()}
	if _, err := b.js.PublishMsg(ctx, deadLetter); err != nil {
		return fmt.Errorf("publish NATS dead-letter envelope: %w", err)
	}
	return nil
}

// Stats maps consumer and stream counters to the queue counters: pending messages are Ready,
// scheduled messages under the schedule subject are Deferred, and unacknowledged envelopes are
// InFlight.
func (b *Driver[T]) Stats(ctx context.Context) (queue.Stats, error) {
	info, err := b.consumer.Info(ctx)
	if err != nil {
		return queue.Stats{}, err
	}
	if b.config.DisableScheduling {
		return queue.Stats{Ready: queueCount(info.NumPending), InFlight: int64(info.NumAckPending)}, nil
	}
	streamInfo, err := b.stream.Info(ctx, jetstream.WithSubjectFilter(b.config.ScheduleSubject+".>"))
	if err != nil {
		return queue.Stats{}, err
	}
	var deferred uint64
	for _, count := range streamInfo.State.Subjects {
		deferred += count
	}
	return queue.Stats{
		Ready: queueCount(info.NumPending), Deferred: queueCount(deferred), InFlight: int64(info.NumAckPending),
	}, nil
}

func queueCount(value uint64) int64 {
	return int64(min(value, uint64(math.MaxInt64)))
}

// Close stops dequeue-polling. The stream, consumer, and the shared client connection stay intact so
// other users of the stream are unaffected.
func (b *Driver[T]) Close(context.Context) error {
	b.closed.Store(true)
	b.cancel()
	return nil
}

type jobEnvelope[T any] struct {
	driver   *Driver[T]
	message  jetstream.Msg
	value    T
	metadata queue.Metadata
	settled  atomic.Bool
}

func (d *jobEnvelope[T]) Value() T {
	return d.value
}

func (d *jobEnvelope[T]) Metadata() queue.Metadata {
	return d.metadata
}

func (d *jobEnvelope[T]) Settled() bool {
	return d.settled.Load()
}
func (d *jobEnvelope[T]) terminal(operation func() error) error {
	if !d.settled.CompareAndSwap(false, true) {
		return queue.ErrJobEnvelopeSettled
	}
	if err := operation(); err != nil {
		d.settled.Store(false)
		return err
	}
	return nil
}
func (d *jobEnvelope[T]) Ack(ctx context.Context) error {
	return d.terminal(func() error { return d.message.DoubleAck(ctx) })
}
func (d *jobEnvelope[T]) Requeue(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.terminal(func() error {
		if delay > 0 {
			return d.message.NakWithDelay(delay)
		}
		return d.message.Nak()
	})
}
func (d *jobEnvelope[T]) Reject(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.terminal(d.message.Term)
}
func (d *jobEnvelope[T]) Touch(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.Settled() {
		return queue.ErrJobEnvelopeSettled
	}
	return d.message.InProgress()
}

// DeadLetter preserves the original NATS message with the supplied cause and then terminates it.
func (d *jobEnvelope[T]) DeadLetter(ctx context.Context, cause error) error {
	if cause == nil {
		cause = errors.New("envelope rejected")
	}
	return d.terminal(func() error {
		if err := d.driver.publishDeadLetter(ctx, d.message, cause); err != nil {
			return err
		}
		return d.message.Term()
	})
}

var _ queue.Driver[int] = (*Driver[int])(nil)

func validateSubjects(patterns []string, subject, scheduleSubject string) error {
	if !coveredBy(patterns, subject) {
		return fmt.Errorf("NATS stream does not cover queue subject %q", subject)
	}
	if scheduleSubject != "" && !coveredBy(patterns, scheduleSubject+".probe") {
		return fmt.Errorf("NATS stream does not cover schedule subject %q", scheduleSubject+".>")
	}
	return nil
}

func coveredBy(patterns []string, subject string) bool {
	subjectTokens := strings.Split(subject, ".")
	for _, pattern := range patterns {
		patternTokens := strings.Split(pattern, ".")
		matched := true
		for index, token := range patternTokens {
			if token == ">" {
				if matched && index < len(subjectTokens) {
					return true
				}
				break
			}
			if index >= len(subjectTokens) || token != "*" && token != subjectTokens[index] {
				matched = false
				break
			}
		}
		if matched && len(patternTokens) == len(subjectTokens) {
			return true
		}
	}
	return false
}
