package core

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/retailcrm/mg-transport-core/v2/core/util/testutil"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/suite"
	"golang.org/x/text/language"

	"github.com/retailcrm/mg-transport-core/v2/core/config"
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
	log := testutil.NewBufferedLogger()
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
	log := testutil.NewBufferedLogger()
	conf := config.AWS{Bucket: "bucketName", FolderName: "folder/name"}
	uploader := NewModuleFeaturesUploader(log, conf, t.localizer, "filename.txt")
	content := "test content " + t.localizer.GetLocalizedMessage("message")

	res, err := uploader.translate([]byte(content), language.Russian)
	t.Assert().Nil(err)
	t.Assert().Equal([]byte("test content Test message"), res)
}

func (t *ModuleFeaturesUploaderTest) TestModuleFeaturesUploader_uploadFile() {
	log := testutil.NewBufferedLogger()
	conf := config.AWS{Bucket: "bucketName", FolderName: "folder/name"}
	uploader := NewModuleFeaturesUploader(log, conf, t.localizer, "source.md")
	content := "test content"

	uploader.client = mockPutObjectAPI(t.T(), "bucketName", "folder/name/filename")
	resp, err := uploader.uploadFile([]byte(content), "filename")
	t.Assert().Equal("https://s3.local/folder/file", resp.Location)
	t.Assert().Nil(err)
}

type uploader func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(uploader *manager.Uploader)) (*manager.UploadOutput, error)

func (m uploader) Upload(ctx context.Context, params *s3.PutObjectInput, optFns ...func(uploader *manager.Uploader)) (*manager.UploadOutput, error) {
	return m(ctx, params, optFns...)
}

func mockPutObjectAPI(t *testing.T, bucket, key string) IUploader {
	return uploader(func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(uploader *manager.Uploader)) (*manager.UploadOutput, error) {
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

		return &manager.UploadOutput{Location: "https://s3.local/folder/file"}, nil
	})
}
