package util

import (
	"bytes"
	// nolint:gosec
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gin-gonic/gin"
	retailcrm "github.com/retailcrm/api-client-go/v2"
	"github.com/retailcrm/mg-transport-core/v2/core/config"

	v1 "github.com/retailcrm/mg-transport-api-client-go/v1"

	"github.com/retailcrm/mg-transport-core/v2/core/logger"

	"github.com/retailcrm/mg-transport-core/v2/core/util/errorutil"
)

var (
	markdownSymbols = []string{"*", "_", "`", "["}
	slashRegex      = regexp.MustCompile(`/+$`)
)

var (
	DefaultScopes = []string{
		"integration_read",
		"integration_write",
	}
	DefaultCredentials = []string{
		"/api/integration-modules/{code}",
		"/api/integration-modules/{code}/edit",
	}
)

var lHSCurrencies = []string{"pen"}

var defaultCurrencies = map[string]string{
	"rub": "₽",
	"uah": "₴",
	"byn": "Br",
	"kzt": "₸",
	"usd": "$",
	"eur": "€",
	"prb": "PRB",
	"mdl": "L",
	"kgs": "с",
	"pln": "zł",
	"azn": "₼",
	"amd": "֏",
	"thb": "฿",
	"aed": "AED",
	"nok": "kr",
	"cad": "C$",
	"czk": "Kč",
	"sek": "kr",
	"dkk": "kr",
	"ron": "lei",
	"uzs": "So'm",
	"aud": "$",
	"chf": "₣",
	"inr": "₹",
	"bgn": "лв",
	"ngn": "₦",
	"huf": "ƒ",
	"ils": "₪",
	"try": "₺",
	"stn": "₡",
	"ars": "$",
	"bob": "Bs",
	"ves": "Bs",
	"gtq": "Q",
	"hnl": "L",
	"dop": "RD$",
	"cop": "COL$",
	"crc": "₡",
	"cup": "$MN",
	"nio": "C$",
	"pab": "B/",
	"pyg": "₲",
	"pen": "S/",
	"svc": "₡",
	"uyu": "$U",
	"gel": "₾",
	"gbp": "£",
}

// Utils service object.
type Utils struct {
	Logger       logger.Logger
	slashRegex   *regexp.Regexp
	AWS          config.AWS
	TokenCounter uint32
	IsDebug      bool
}

// NewUtils will create new Utils instance.
func NewUtils(awsConfig config.AWS, logger logger.Logger, debug bool) *Utils {
	return &Utils{
		IsDebug:      debug,
		AWS:          awsConfig,
		Logger:       logger,
		TokenCounter: 0,
		slashRegex:   slashRegex,
	}
}

// ResetUtils resets the utils inner state.
func (u *Utils) ResetUtils(awsConfig config.AWS, debug bool, tokenCounter uint32) {
	u.TokenCounter = tokenCounter
	u.AWS = awsConfig
	u.IsDebug = debug
	u.slashRegex = slashRegex
}

// GenerateToken will generate long pseudo-random string.
func (u *Utils) GenerateToken() string {
	c := atomic.AddUint32(&u.TokenCounter, 1)

	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d%d", time.Now().UnixNano(), c))))
}

// GetAPIClient will initialize RetailCRM api client from url and key.
// Scopes will be used to determine if client is valid. If there are no scopes - credentials will be used instead.
func (u *Utils) GetAPIClient(
	url, key string, scopes []string, credentials ...[]string) (*retailcrm.Client, int, error) {
	client := retailcrm.New(url, key).
		WithLogger(logger.APIClientAdapter(u.Logger))
	client.Debug = u.IsDebug

	cr, status, err := client.APICredentials()
	if err != nil {
		return nil, status, err
	}

	if res := u.checkScopes(cr.Scopes, scopes); len(res) != 0 {
		if len(credentials) == 0 || len(cr.Scopes) > 0 {
			u.Logger.Error(url, logger.HTTPStatusCode(status), logger.Body(res))
			return nil, http.StatusBadRequest, errorutil.NewInsufficientScopesErr(res)
		}

		if res := u.checkScopes(cr.Credentials, credentials[0]); len(res) != 0 {
			u.Logger.Error(url, logger.HTTPStatusCode(status), logger.Body(res))
			return nil, http.StatusBadRequest, errorutil.NewInsufficientScopesErr(res)
		}
	}

	return client, 0, nil
}

func (u *Utils) checkScopes(scopes []string, scopesRequired []string) []string {
	rs := make([]string, len(scopesRequired))
	copy(rs, scopesRequired)

	for _, vs := range scopes {
		for kn, vn := range rs {
			if vn == vs {
				if len(rs) == 1 {
					rs = rs[:0]
					break
				}
				rs = append(rs[:kn], rs[kn+1:]...)
			}
		}
	}

	return rs
}

// UploadUserAvatar will upload avatar for user.
func (u *Utils) UploadUserAvatar(url string) (picURLs3 string, err error) {
	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(
			u.AWS.AccessKeyID,
			u.AWS.SecretAccessKey,
			""),
		Region: aws.String(u.AWS.Region),
	}

	if u.AWS.Endpoint != "" {
		s3Config.Endpoint = aws.String(u.AWS.Endpoint)
	}

	s := session.Must(session.NewSession(s3Config))
	uploader := s3manager.NewUploader(s)

	// nolint:gosec
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("get: %v code: %v", url, resp.StatusCode)
	}

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(u.AWS.Bucket),
		Key:         aws.String(fmt.Sprintf("%v/%v.jpg", u.AWS.FolderName, u.GenerateToken())),
		Body:        resp.Body,
		ContentType: aws.String(u.AWS.ContentType),
		ACL:         aws.String("public-read"),
	})
	if err != nil {
		return
	}

	picURLs3 = result.Location

	return
}

// RemoveTrailingSlash will remove slash at the end of any string.
func (u *Utils) RemoveTrailingSlash(crmURL string) string {
	return u.slashRegex.ReplaceAllString(crmURL, ``)
}

// GetMGItemData will upload file to MG by URL and return information about attachable item.
func GetMGItemData(client *v1.MgClient, url string, caption string) (v1.Item, int, error) {
	item := v1.Item{}

	data, st, err := client.UploadFileByURL(
		v1.UploadFileByUrlRequest{
			Url: url,
		},
	)
	if err != nil {
		return item, st, err
	}

	item.ID = data.ID
	item.Caption = caption

	return item, st, err
}

// GetEntitySHA1 will serialize any value to JSON and return SHA1 hash of this JSON.
func GetEntitySHA1(v interface{}) (hash string, err error) {
	res, _ := json.Marshal(v)

	// nolint:gosec
	h := sha1.New()
	_, err = h.Write(res)
	hash = fmt.Sprintf("%x", h.Sum(nil))

	return
}

// ReplaceMarkdownSymbols will remove markdown symbols from text.
func ReplaceMarkdownSymbols(s string) string {
	for _, v := range markdownSymbols {
		s = strings.ReplaceAll(s, v, "\\"+v)
	}

	return s
}

// DefaultCurrencies will return default currencies list for all bots.
func DefaultCurrencies() map[string]string {
	return defaultCurrencies
}

// LHSCurrencies will return left-hand side currencies.
func LHSCurrencies() []string {
	return lHSCurrencies
}

// GetCurrencySymbol returns currency symbol by it's ISO 4127 code.
// It returns provided currency code in uppercase if currency symbol cannot be found.
func GetCurrencySymbol(code string) string {
	if i, ok := DefaultCurrencies()[strings.ToLower(code)]; ok {
		return i
	}

	return strings.ToUpper(code)
}

// IsLHSCurrency determines whether the currency is left-hand side.
// It returns false if currency symbol cannot be found.
func IsLHSCurrency(code string) bool {
	return slices.Contains(LHSCurrencies(), strings.ToLower(code))
}

func FormatCurrencyValue(value float32) string {
	return fmt.Sprintf("%.2f", value)
}

// BindJSONWithRaw will perform usual ShouldBindJSON and will return the original body data.
func BindJSONWithRaw(c *gin.Context, obj any) ([]byte, error) {
	closer := c.Request.Body
	defer func() { _ = closer.Close() }()
	data, err := io.ReadAll(closer)
	if err != nil {
		return []byte{}, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	return data, c.ShouldBindJSON(obj)
}
