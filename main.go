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
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"github.com/joeshaw/envdecode"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

const (
	browserContextTimeout = 10 * time.Minute
	browserWindowWidth    = 1440
	browserWindowHeight   = 900
)

func main() {
	config, err := initConfig()
	if err != nil {
		log.Panic(err)
	}

	if config.Schedule == "" {
		err = run(config)
		if err != nil {
			log.Panic(err)
		}
	} else {
		err = schedule(config.Schedule, func() {
			err = run(config)
			if err != nil {
				log.Panic(err)
			}
		})
		if err != nil {
			log.Panic(err)
		}
	}
}

func run(config Config) error {
	err := initScreenshotsPath(config.Browser.ScreenshotsPath)
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
		config.Zoho.CompanyID,
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

	return nil
}

func schedule(schedule string, job func()) error {
	c := cron.New(
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
		),
		cron.WithLocation(time.UTC),
	)

	eid, err := c.AddFunc(schedule, job)
	if err != nil {
		return err
	}

	_, err = c.AddFunc(schedule, func() {
		log.Printf("Next launch scheduled for: %s", c.Entry(eid).Schedule.Next(time.Now().UTC()))
	})
	if err != nil {
		return err
	}

	log.Printf("Launch scheduled for: %s", c.Entry(eid).Schedule.Next(time.Now().UTC()))

	c.Run()

	return nil
}

func initBrowserContext(execPath string, headless bool) (context.Context, context.CancelFunc) {
	var opts []chromedp.ExecAllocatorOption

	opts = append(
		opts,
		chromedp.ExecPath(execPath),
		chromedp.NoSandbox,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.WindowSize(browserWindowWidth, browserWindowHeight),
		chromedp.Flag("disable-software-rasterizer", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	if headless {
		opts = append(opts, chromedp.Headless)
	}

	ctx, cancel1 := context.WithTimeout(context.Background(), browserContextTimeout)

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

func login(ctx context.Context, username, password, companyID string, screenshotsPath string) error {
	screenshotsData := make([][]byte, 100)
	defer func() {
		err := saveScreenshots(screenshotsPath, "login", screenshotsData)
		if err != nil {
			log.Printf("cannot save screenshots: %s", err)
		}
	}()

	log.Println("Login: started")

	geolocationPermissionDescriptor := browser.PermissionDescriptor{
		Name: browser.PermissionTypeGeolocation.String(),
	}

	loginURL := "https://accounts.zoho.eu/signin?servicename=zohopeople"
	dashboardURL := fmt.Sprintf("https://people.zoho.eu/%s/zp#home/dashboard", companyID)

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			err := browser.SetPermission(&geolocationPermissionDescriptor, browser.PermissionSettingDenied).Do(ctx)
			if err != nil {
				return err
			}

			// Navigate to login page
			err = chromedp.Navigate(loginURL).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Login: login page loaded")

			err = chromedp.CaptureScreenshot(&screenshotsData[0]).Do(ctx)
			if err != nil {
				return err
			}

			// Load sign-in methods page
			err = chromedp.Click(".fed_2show_small", chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Login: sign-in methods loaded")

			err = chromedp.CaptureScreenshot(&screenshotsData[1]).Do(ctx)
			if err != nil {
				return err
			}

			// Select sign-in method
			err = chromedp.Click("span[title='Sign in using Microsoft']", chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Login: sign-in method selected")

			// Enter username
			err = chromedp.SetValue("#i0116", username, chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.Sleep(5 * time.Second).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.CaptureScreenshot(&screenshotsData[2]).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.Click("#idSIButton9", chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Login: username entered")

			// Put password
			err = chromedp.SetValue("#i0118", password, chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.Sleep(5 * time.Second).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.CaptureScreenshot(&screenshotsData[3]).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.Click("#idSIButton9", chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Login: password entered")

			// Confirm stay signed-in
			err = chromedp.Sleep(5 * time.Second).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.CaptureScreenshot(&screenshotsData[4]).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.Click("#idSIButton9", chromedp.NodeVisible).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Login: stay signed-in choice confirmed")

			// Wait for redirect
			err = chromedp.WaitReady("body").Do(ctx)
			if err != nil {
				return err
			}

			// Navigate to dashboard
			err = chromedp.Navigate(dashboardURL).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Login: dashboard page loaded")

			err = chromedp.Sleep(5 * time.Second).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.CaptureScreenshot(&screenshotsData[5]).Do(ctx)
			if err != nil {
				return err
			}

			// Wait for profile image
			err = chromedp.WaitVisible("#zpeople_userimage").Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Login: profile image loaded")
			log.Println("Login: logged in")

			return nil
		}),
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

	geolocationPermissionDescriptor := browser.PermissionDescriptor{
		Name: browser.PermissionTypeGeolocation.String(),
	}

	attendanceURL := fmt.Sprintf("https://people.zoho.eu/%s/zp#attendance/entry/listview", companyID)

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var checkInButtonHTML string

			err := browser.SetPermission(&geolocationPermissionDescriptor, browser.PermissionSettingDenied).Do(ctx)
			if err != nil {
				return err
			}

			// Navigate to attendance page
			err = chromedp.Navigate(attendanceURL).Do(ctx)
			if err != nil {
				return err
			}

			log.Println("Check-in: attendance page loaded")

			err = chromedp.Sleep(5 * time.Second).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.CaptureScreenshot(&screenshotsData[0]).Do(ctx)
			if err != nil {
				return err
			}

			// Parse check-in state
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

			// Check-in
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

	flag.StringVar(
		&config.Browser.ExecPath,
		"e",
		config.Browser.ExecPath,
		"Chrome executable path",
	)

	flag.StringVar(
		&config.Browser.ScreenshotsPath,
		"s",
		config.Browser.ScreenshotsPath,
		"Screenshots path",
	)

	flag.BoolVar(
		&config.Browser.Headless,
		"h",
		config.Browser.Headless,
		"Headless mode",
	)

	flag.StringVar(
		&config.Zoho.Username,
		"u",
		config.Zoho.Username,
		"Zoho username",
	)

	flag.StringVar(
		&config.Zoho.Password,
		"p",
		config.Zoho.Password,
		"Zoho password",
	)

	flag.StringVar(
		&config.Zoho.CompanyID,
		"c",
		config.Zoho.CompanyID,
		"Zoho company identifier",
	)

	flag.StringVar(
		&config.Schedule,
		"x",
		config.Schedule,
		"Schedule of check-in launches using CRON format and UTC time",
	)

	flag.Parse()

	err = config.Validate()
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
