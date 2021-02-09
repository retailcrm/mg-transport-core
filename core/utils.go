package core

import (
	// nolint:gosec
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	v5 "github.com/retailcrm/api-client-go/v5"
	v1 "github.com/retailcrm/mg-transport-api-client-go/v1"
)

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
	"mxn": "NP$",
	"nio": "C$",
	"pab": "B/",
	"pyg": "₲",
	"pen": "S/",
	"svc": "₡",
	"uyu": "$U",
	"clp": "Ch$",
	"gel": "₾",
	"gbp": "£",
}

// Utils service object.
type Utils struct {
	IsDebug      bool
	TokenCounter uint32
	ConfigAWS    ConfigAWS
	Logger       LoggerInterface
	slashRegex   *regexp.Regexp
}

// NewUtils will create new Utils instance.
func NewUtils(awsConfig ConfigAWS, logger LoggerInterface, debug bool) *Utils {
	return &Utils{
		IsDebug:      debug,
		ConfigAWS:    awsConfig,
		Logger:       logger,
		TokenCounter: 0,
		slashRegex:   slashRegex,
	}
}

// resetUtils.
func (u *Utils) resetUtils(awsConfig ConfigAWS, debug bool, tokenCounter uint32) {
	u.TokenCounter = tokenCounter
	u.ConfigAWS = awsConfig
	u.IsDebug = debug
	u.slashRegex = slashRegex
}

// GenerateToken will generate long pseudo-random string.
func (u *Utils) GenerateToken() string {
	c := atomic.AddUint32(&u.TokenCounter, 1)

	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d%d", time.Now().UnixNano(), c))))
}

// GetAPIClient will initialize RetailCRM api client from url and key.
func (u *Utils) GetAPIClient(url, key string) (*v5.Client, int, error) {
	client := v5.New(url, key)
	client.Debug = u.IsDebug

	cr, status, e := client.APICredentials()
	if e != nil && e.Error() != "" {
		u.Logger.Error(url, status, e.Error(), cr)
		return nil, http.StatusInternalServerError, errors.New(e.Error())
	}

	if !cr.Success {
		errMsg := "unknown error"

		if e != nil {
			if e.ApiError() != "" {
				errMsg = e.ApiError()
			} else if e.ApiErrors() != nil {
				errMsg = ""

				for key, errText := range e.ApiErrors() {
					errMsg += fmt.Sprintf("[%s: %s] ", key, errText)
				}
			}
		}

		u.Logger.Error(url, status, errMsg, cr)
		return nil, http.StatusBadRequest, errors.New("invalid credentials")
	}

	if res := u.checkCredentials(cr.Credentials); len(res) != 0 {
		u.Logger.Error(url, status, res)
		return nil, http.StatusBadRequest, errors.New("missing credentials")
	}

	return client, 0, nil
}

func (u *Utils) checkCredentials(credential []string) []string {
	rc := make([]string, len(credentialsTransport))
	copy(rc, credentialsTransport)

	for _, vc := range credential {
		for kn, vn := range rc {
			if vn == vc {
				if len(rc) == 1 {
					rc = rc[:0]
					break
				}
				rc = append(rc[:kn], rc[kn+1:]...)
			}
		}
	}

	return rc
}

// UploadUserAvatar will upload avatar for user.
func (u *Utils) UploadUserAvatar(url string) (picURLs3 string, err error) {
	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(
			u.ConfigAWS.AccessKeyID,
			u.ConfigAWS.SecretAccessKey,
			""),
		Region: aws.String(u.ConfigAWS.Region),
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
		Bucket:      aws.String(u.ConfigAWS.Bucket),
		Key:         aws.String(fmt.Sprintf("%v/%v.jpg", u.ConfigAWS.FolderName, u.GenerateToken())),
		Body:        resp.Body,
		ContentType: aws.String(u.ConfigAWS.ContentType),
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
		s = strings.Replace(s, v, "\\"+v, -1)
	}

	return s
}

// DefaultCurrencies will return default currencies list for all bots.
func DefaultCurrencies() map[string]string {
	return defaultCurrencies
}

// GetCurrencySymbol returns currency symbol by it's ISO 4127 code.
// It returns provided currency code in uppercase if currency symbol cannot be found.
func GetCurrencySymbol(code string) string {
	if i, ok := DefaultCurrencies()[strings.ToLower(code)]; ok {
		return i
	}

	return strings.ToUpper(code)
}
