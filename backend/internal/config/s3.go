package config

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func (s S3Config) Validate() error {
	if s.Enabled && (!bucketName.MatchString(s.Bucket) || strings.Contains(s.Bucket, "..") || strings.Contains(s.Bucket, ".-") || strings.Contains(s.Bucket, "-.") || net.ParseIP(s.Bucket) != nil) {
		return fmt.Errorf("无效的 S3 bucket")
	}
	if (s.AccessKey == "") != (s.SecretKey == "") {
		return fmt.Errorf("S3 access_key 和 secret_key 必须成对配置")
	}
	_, err := s.EndpointURL()
	return err
}

func (s S3Config) EndpointURL() (string, error) {
	endpoint := s.Endpoint
	if endpoint == "" {
		return "", nil
	}
	if !strings.Contains(endpoint, "://") {
		scheme := "https"
		if !s.UseSSL {
			scheme = "http"
		}
		endpoint = scheme + "://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", fmt.Errorf("无效的 S3 endpoint，必须为 HTTP 或 HTTPS 地址")
	}
	return endpoint, nil
}
