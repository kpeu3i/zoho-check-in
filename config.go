package main

import "errors"

type Config struct {
	Browser struct {
		ExecPath        string `env:"ZP_BROWSER_EXEC_PATH"`
		ScreenshotsPath string `env:"ZP_BROWSER_SCREENSHOTS_PATH"`
		Headless        bool   `env:"ZP_BROWSER_HEADLESS"`
	}
	Zoho struct {
		Username  string `env:"ZP_ZOHO_USERNAME"`
		Password  string `env:"ZP_ZOHO_PASSWORD"`
		CompanyID string `env:"ZP_ZOHO_COMPANY_ID"`
	}
}

func (c *Config) Validate() error {
	if c.Browser.ExecPath == "" {
		return errors.New("config: browser executable path is required")
	}

	if c.Zoho.Username == "" {
		return errors.New("config: zoho username is required")
	}

	if c.Zoho.Password == "" {
		return errors.New("config: zoho password is required")
	}

	if c.Zoho.CompanyID == "" {
		return errors.New("config: zoho company id is required")
	}

	return nil
}
