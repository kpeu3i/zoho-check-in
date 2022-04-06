package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/joeshaw/envdecode"
	"github.com/joho/godotenv"
)

func main() {
	config, err := initConfig()
	if err != nil {
		log.Panic(err)
	}

	err = initScreenshotsPath(config.Browser.ScreenshotsPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := initBrowserContext(config.Browser.ExecPath, config.Browser.Headless)
	defer cancel()

	defer func() {
		err = logout(ctx)
		if err != nil {
			log.Println(err)
		}
	}()

	err = login(
		ctx,
		config.Zoho.Username,
		config.Zoho.Password,
		config.Browser.ScreenshotsPath,
	)
	if err != nil {
		log.Panic(err)
	}

	err = checkIn(
		ctx,
		config.Zoho.CompanyID,
		config.Browser.ScreenshotsPath,
	)
	if err != nil {
		log.Panic(err)
	}
}

func initBrowserContext(execPath string, headless bool) (context.Context, context.CancelFunc) {
	var opts []chromedp.ExecAllocatorOption

	opts = append(
		opts,
		chromedp.ExecPath(execPath),
		chromedp.NoSandbox,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.WindowSize(1440, 900),
		chromedp.DisableGPU,
		chromedp.Flag("disable-software-rasterizer", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	if headless {
		opts = append(opts, chromedp.Headless)
	}

	ctx, cancel1 := context.WithTimeout(context.Background(), 5*time.Minute)

	ctx, cancel2 := chromedp.NewExecAllocator(
		ctx,
		opts...,
	)

	ctx, cancel3 := chromedp.NewContext(
		ctx,
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	)

	cancelFunc := func() {
		cancel3()
		cancel2()
		cancel1()
	}

	return ctx, cancelFunc
}

func login(ctx context.Context, username, password string, screenshotsPath string) error {
	screenshotsData := make([][]byte, 100)
	defer func() {
		err := saveScreenshots(screenshotsPath, "login", screenshotsData)
		if err != nil {
			log.Printf("cannot save screenshots: %s", err)
		}
	}()

	log.Println("Login: started")

	err := chromedp.Run(ctx,
		chromedp.Navigate("https://accounts.zoho.eu/signin?servicename=zohopeople"),
		chromedp.CaptureScreenshot(&screenshotsData[0]),

		// Pick sign-in method
		chromedp.Click("span[title='Sign in using Microsoft']", chromedp.NodeVisible),

		// Put username
		chromedp.SetValue("#i0116", username, chromedp.NodeVisible),
		chromedp.Sleep(5*time.Second),
		chromedp.CaptureScreenshot(&screenshotsData[1]),
		chromedp.Click("#idSIButton9", chromedp.NodeVisible),

		// Put password
		chromedp.SetValue("#i0118", password, chromedp.NodeVisible),
		chromedp.Sleep(5*time.Second),
		chromedp.CaptureScreenshot(&screenshotsData[2]),
		chromedp.Click("#idSIButton9", chromedp.NodeVisible),

		// Confirm stay signed-in
		chromedp.Sleep(5*time.Second),
		chromedp.CaptureScreenshot(&screenshotsData[3]),
		chromedp.Click("#idSIButton9", chromedp.NodeVisible),

		// Wait for redirect
		chromedp.WaitReady("body"),
		chromedp.Sleep(5*time.Second),
		chromedp.CaptureScreenshot(&screenshotsData[4]),
	)
	if err != nil {
		return err
	}

	log.Println("Login: completed")

	return nil
}

func checkIn(ctx context.Context, companyID string, screenshotsPath string) error {
	screenshotsData := make([][]byte, 100)
	defer func() {
		err := saveScreenshots(screenshotsPath, "checkin", screenshotsData)
		if err != nil {
			log.Printf("cannot save screenshots: %s", err)
		}
	}()

	log.Println("Check-in: started")

	attendanceURL := fmt.Sprintf("https://people.zoho.eu/%s/zp#attendance/entry/listview", companyID)

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var checkInButtonHTML string

			err := chromedp.Navigate(attendanceURL).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Check-in: attendance page loaded")

			err = chromedp.Sleep(5 * time.Second).Do(ctx)
			if err != nil {
				return err
			}

			// **************************************
			err = chromedp.CaptureScreenshot(&screenshotsData[0]).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.OuterHTML("#ZPAtt_check_in_out", &checkInButtonHTML, chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Check-in: check-in button HTML extracted")

			isCheckedIn, err := parseCheckInState(checkInButtonHTML)
			if err != nil {
				return err
			}

			if isCheckedIn {
				log.Println("Check-in: checked-in")

				return nil
			}

			// **************************************
			err = chromedp.Click("#ZPAtt_check_in_out", chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Check-in: check-in button clicked")

			err = chromedp.Sleep(5 * time.Second).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.CaptureScreenshot(&screenshotsData[1]).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.OuterHTML("#ZPAtt_check_in_out", &checkInButtonHTML, chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Check-in: check-in button HTML extracted")

			isCheckedIn, err = parseCheckInState(checkInButtonHTML)
			if err != nil {
				return err
			}

			if isCheckedIn {
				log.Println("Check-in: checked-in")

				return nil
			}

			// **************************************
			err = chromedp.Focus("#ZPAtt_check_in_out", chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Check-in: check-in button focused")

			err = chromedp.Click("#ZPAtt_check_in_out", chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Check-in: check-in button clicked")

			err = chromedp.Sleep(5 * time.Second).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.CaptureScreenshot(&screenshotsData[2]).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.OuterHTML("#ZPAtt_check_in_out", &checkInButtonHTML, chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Check-in: check-in button HTML extracted")

			isCheckedIn, err = parseCheckInState(checkInButtonHTML)
			if err != nil {
				return err
			}

			if !isCheckedIn {
				return errors.New("cannot check-in")
			}

			log.Println("Check-in: checked-in")

			return nil
		}),
	)
	if err != nil {
		return err
	}

	log.Println("Check-in: completed")

	return nil
}

func logout(ctx context.Context) error {
	log.Println("Logout: started")

	err := chromedp.Run(ctx,
		chromedp.Navigate("https://people.zoho.eu/Logout.do"),
	)
	if err != nil {
		return err
	}

	log.Println("Logout: completed")

	return nil
}

func parseCheckInState(buttonHTML string) (bool, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(buttonHTML))
	if err != nil {
		return false, err
	}

	if doc.Find(".grn-bg").Length() == 1 {
		return false, nil
	}

	if doc.Find(".red-bg").Length() == 1 {
		return true, nil
	}

	return false, errors.New("cannot resolve check-in state")
}

func initScreenshotsPath(screenshotsPath string) error {
	if screenshotsPath == "" {
		return nil
	}

	err := os.MkdirAll(screenshotsPath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	files, err := filepath.Glob(path.Join(screenshotsPath, "zpcheckin_step_*.png"))
	if err != nil {
		return err
	}

	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			return err
		}
	}

	return nil
}

func saveScreenshots(screenshotsPath string, namespace string, screenshotsData [][]byte) error {
	if screenshotsPath == "" {
		return nil
	}

	for i, data := range screenshotsData {
		if len(data) == 0 {
			continue
		}

		err := ioutil.WriteFile(
			path.Join(screenshotsPath, fmt.Sprintf("zpcheckin_step_%s_%d.png", namespace, i+1)),
			data,
			0644,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func initConfig() (Config, error) {
	config := Config{}
	config.Browser.ExecPath = "chromium-browser"
	config.Browser.ScreenshotsPath = "/zpcheckin/screenshots"
	config.Browser.Headless = true

	err := godotenv.Load()
	if err != nil {
		var fsErr *fs.PathError
		if !errors.As(err, &fsErr) {
			return Config{}, err
		}
	}

	err = envdecode.Decode(&config)
	if err != nil {
		if err != envdecode.ErrNoTargetFieldsAreSet {
			return Config{}, err
		}
	}

	flag.StringVar(&config.Browser.ExecPath, "e", config.Browser.ExecPath, "Chrome executable path")
	flag.StringVar(&config.Browser.ScreenshotsPath, "s", config.Browser.ScreenshotsPath, "Screenshots path")
	flag.BoolVar(&config.Browser.Headless, "h", config.Browser.Headless, "Headless mode")
	flag.StringVar(&config.Zoho.Username, "u", config.Zoho.Username, "Zoho username")
	flag.StringVar(&config.Zoho.Password, "p", config.Zoho.Password, "Zoho password")
	flag.StringVar(&config.Zoho.CompanyID, "c", config.Zoho.CompanyID, "Zoho company identifier")

	flag.Parse()

	err = config.Validate()
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
