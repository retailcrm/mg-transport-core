package core

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gomarkdown/markdown"
	"golang.org/x/text/language"

	"github.com/retailcrm/mg-transport-core/v2/core/config"
	"github.com/retailcrm/mg-transport-core/v2/core/logger"
)

type ModuleFeaturesUploader struct {
	client           *s3.Client
	log              logger.Logger
	loc              LocalizerInterface
	bucket           string
	folder           string
	featuresFilename string
}

var languages = []language.Tag{language.Russian, language.English, language.Spanish}

func NewModuleFeaturesUploader(
	log logger.Logger,
	conf config.AWS,
	translateFs fs.FS,
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
		log.Fatal(err)
	}

	return &ModuleFeaturesUploader{
		client:           s3.NewFromConfig(cfg),
		log:              log,
		loc:              NewLocalizerFS(DefaultLanguage, DefaultLocalizerMatcher(), translateFs),
		bucket:           conf.Bucket,
		folder:           conf.FolderName,
		featuresFilename: featuresFilename,
	}
}

func (s *ModuleFeaturesUploader) Upload() {
	s.log.Debugf("upload module features started...")

	content, err := os.ReadFile(s.featuresFilename)
	if err != nil {
		s.log.Errorf("cannot read markdown file %s %s", s.featuresFilename, err.Error())
		return
	}

	for _, lang := range languages {
		translated, err := s.translate(content, lang)
		if err != nil {
			s.log.Errorf("cannot translate module features file to %s: %s", lang.String(), err.Error())
			continue
		}

		html := markdown.ToHTML(translated, nil, nil)

		if err := s.uploadFile(html, lang.String()); err != nil {
			s.log.Errorf("cannot upload file %s: %s", lang.String(), err.Error())
			continue
		}
	}

	s.log.Debugf("upload module features finished")
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

func (s *ModuleFeaturesUploader) uploadFile(content []byte, filename string) error {
	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fmt.Sprintf("%s/%s.html", s.folder, filename)),
		Body:   bytes.NewReader(content),
	})

	return err
}
