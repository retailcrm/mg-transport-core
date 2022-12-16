package core

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/op/go-logging"
	"github.com/stretchr/testify/suite"
	"golang.org/x/text/language"

	"github.com/retailcrm/mg-transport-core/v2/core/config"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

type ModuleFeaturesUploaderTest struct {
	suite.Suite
	localizer LocalizerInterface
}

func TestModuleFeaturesUploader(t *testing.T) {
	suite.Run(t, new(ModuleFeaturesUploaderTest))
}

func (t *ModuleFeaturesUploaderTest) SetupSuite() {
	createTestLangFiles(t.T())
	t.localizer = NewLocalizer(language.English, DefaultLocalizerMatcher(), testTranslationsDir)
}

func (t *ModuleFeaturesUploaderTest) TearDownSuite() {
	err := os.RemoveAll(testTranslationsDir)
	t.Require().Nil(err)
}

func (t *ModuleFeaturesUploaderTest) TestModuleFeaturesUploader_NewModuleFeaturesUploader() {
	logs := &bytes.Buffer{}
	log := logger.NewBase(logs, "code", logging.DEBUG, logger.DefaultLogFormatter())
	conf := config.AWS{Bucket: "bucketName", FolderName: "folder/name"}

	uploader := NewModuleFeaturesUploader(log, conf, t.localizer, "filename.txt")
	t.Assert().NotNil(uploader)
	t.Assert().NotNil(uploader.client)
	t.Assert().Equal(log, uploader.log)
	t.Assert().NotNil(uploader.loc)
	t.Assert().Equal("bucketName", uploader.bucket)
	t.Assert().Equal("folder/name", uploader.folder)
}

func (t *ModuleFeaturesUploaderTest) TestModuleFeaturesUploader_translate() {
	logs := &bytes.Buffer{}
	log := logger.NewBase(logs, "code", logging.DEBUG, logger.DefaultLogFormatter())
	conf := config.AWS{Bucket: "bucketName", FolderName: "folder/name"}
	uploader := NewModuleFeaturesUploader(log, conf, t.localizer, "filename.txt")
	content := "test content " + t.localizer.GetLocalizedMessage("message")

	res, err := uploader.translate([]byte(content), language.Russian)
	t.Assert().Nil(err)
	t.Assert().Equal([]byte("test content Test message"), res)
}

func (t *ModuleFeaturesUploaderTest) TestModuleFeaturesUploader_uploadFile() {
	logs := &bytes.Buffer{}
	log := logger.NewBase(logs, "code", logging.DEBUG, logger.DefaultLogFormatter())
	conf := config.AWS{Bucket: "bucketName", FolderName: "folder/name"}
	uploader := NewModuleFeaturesUploader(log, conf, t.localizer, "source.md")
	content := "test content"

	uploader.client = mockPutObjectAPI(t.T(), "bucketName", "folder/name/filename.html")
	err := uploader.uploadFile([]byte(content), "filename")
	t.Assert().Nil(err)
}

type putObjectAPI func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)

func (m putObjectAPI) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return m(ctx, params, optFns...)
}

func mockPutObjectAPI(t *testing.T, bucket, key string) S3PutObjectAPI {
	return putObjectAPI(func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
		t.Helper()
		if params.Bucket == nil {
			t.Fatal("expect bucket to not be nil")
		}
		if e, a := bucket, *params.Bucket; e != a {
			t.Errorf("expect %v, got %v", e, a)
		}
		if params.Key == nil {
			t.Fatal("expect key to not be nil")
		}
		if e, a := key, *params.Key; e != a {
			t.Errorf("expect %v, got %v", e, a)
		}

		return &s3.PutObjectOutput{}, nil
	})
}
