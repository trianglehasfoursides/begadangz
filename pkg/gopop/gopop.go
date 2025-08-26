package gopop

import (
	"fmt"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	Username string
	Password string
	Rest     *resty.Client
}

func New(username string, password string) *Client {
	return &Client{
		Username: username,
		Password: password,
		Rest:     resty.New(),
	}
}

func (c *Client) Create(name string, migration string) error {
	res, err := c.Rest.R().
		SetBasicAuth(c.Username, c.Password).
		SetBody(map[string]string{
			"name":      name,
			"migration": migration,
		}).Post("/")

	if err != nil {
		return err
	}

	if res.IsError() {
		return res.Error().(error)
	}

	return nil
}

func (c *Client) Get(name string) (bool, error) {
	name = fmt.Sprintf("/databases/%s", name)
	res, err := c.Rest.R().
		SetBasicAuth(c.Username, c.Password).
		Get(name)

	if err != nil {
		return false, err
	}

	if res.IsError() {
		return false, res.Error().(error)
	}

	return true, nil
}

func (c *Client) Query(name string, query string) error {
	c.Rest.R().
		SetBasicAuth(c.Username, c.Password).
		SetBody(query)
	return nil
}

func (c *Client) Exec(name string, query string) {}

func (c *Client) Drop(name string) {}
