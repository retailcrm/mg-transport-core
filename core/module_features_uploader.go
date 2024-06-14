package core

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gomarkdown/markdown"
	"go.uber.org/zap"
	"golang.org/x/text/language"

	"github.com/retailcrm/mg-transport-core/v2/core/config"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

type IUploader interface {
	Upload(
		ctx context.Context, input *s3.PutObjectInput, optFns ...func(uploader *manager.Uploader),
	) (*manager.UploadOutput, error)
}

type ModuleFeaturesUploader struct {
	client           IUploader
	log              logger.Logger
	loc              LocalizerInterface
	bucket           string
	folder           string
	featuresFilename string
	contentType      string
}

var languages = []language.Tag{language.Russian, language.English, language.Spanish}

func NewModuleFeaturesUploader(
	log logger.Logger,
	conf config.AWS,
	loc LocalizerInterface,
	featuresFilename string,
) *ModuleFeaturesUploader {
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           conf.Endpoint,
				SigningRegion: conf.Region,
			}, nil
		},
	)
	customProvider := aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     conf.AccessKeyID,
			SecretAccessKey: conf.SecretAccessKey,
			CanExpire:       false,
		}, nil
	})

	cfg, err := awsConfig.LoadDefaultConfig(
		context.TODO(),
		awsConfig.WithEndpointResolverWithOptions(customResolver),
		awsConfig.WithCredentialsProvider(customProvider),
	)
	if err != nil {
		log.Error("cannot load S3 configuration", logger.Err(err))
		return nil
	}

	client := manager.NewUploader(s3.NewFromConfig(cfg))
	if err != nil {
		log.Error("cannot load S3 configuration", logger.Err(err))
		return nil
	}

	return &ModuleFeaturesUploader{
		client:           client,
		log:              log,
		loc:              loc,
		bucket:           conf.Bucket,
		folder:           conf.FolderName,
		featuresFilename: featuresFilename,
		contentType:      conf.ContentType,
	}
}

func (s *ModuleFeaturesUploader) Upload() {
	s.log.Debug("upload module features started...")

	content, err := os.ReadFile(s.featuresFilename)
	if err != nil {
		s.log.Error("cannot read markdown file %s %s", zap.String("fileName", s.featuresFilename), logger.Err(err))
		return
	}

	for _, lang := range languages {
		translated, err := s.translate(content, lang)
		if err != nil {
			s.log.Error("cannot translate module features file", zap.String("lang", lang.String()), logger.Err(err))
			continue
		}

		html := markdown.ToHTML(translated, nil, nil)
		resp, err := s.uploadFile(html, lang.String())

		if err != nil {
			s.log.Error("cannot upload file", zap.String("lang", lang.String()), logger.Err(err))
			continue
		}

		fmt.Printf("\nURL of the module specifications file for the %s lang: %s", lang.String(), resp.Location)
	}

	fmt.Println()
	s.log.Debug("upload module features finished")
}

func (s *ModuleFeaturesUploader) translate(content []byte, lang language.Tag) ([]byte, error) {
	s.loc.SetLanguage(lang)
	page := template.Must(template.New("").Funcs(template.FuncMap{
		"trans": s.loc.GetLocalizedMessage,
	}).Parse(string(content)))

	output := &bytes.Buffer{}
	err := page.Execute(output, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func (s *ModuleFeaturesUploader) uploadFile(content []byte, lang string) (*manager.UploadOutput, error) {
	resp, err := s.client.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(fmt.Sprintf("%s/%s", s.folder, lang)),
		Body:        bytes.NewBuffer(content),
		ContentType: aws.String(s.contentType),
		ACL:         "public-read",
	})

	return resp, err
}
