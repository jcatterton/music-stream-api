package service

import (
	"errors"
	"fmt"
	"net/http"
)

type ExternalHandler struct {
	HttpClient      *http.Client
	LoginServiceURL string
}

func (e *ExternalHandler) ValidateToken(token string) error {
	if e.LoginServiceURL == "" {
		return errors.New("login service url cannot be emtpy")
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%v/token", e.LoginServiceURL), nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", token))

	resp, err := e.HttpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("non-200 status code received: %v", resp.StatusCode))
	}

	return nil
}
