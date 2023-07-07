package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lanrat/czds"
)

// flags
var (
	username    = flag.String("username", "", "username to authenticate with")
	password    = flag.String("password", "", "password to authenticate with")
	passin      = flag.String("passin", "", "password source (default: prompt on tty; other options: cmd:command, env:var, file:path, keychain:name, lpass:name, op:name)")
	verbose     = flag.Bool("verbose", false, "enable verbose logging")
	reason      = flag.String("reason", "", "reason to request zone access")
	printTerms  = flag.Bool("terms", false, "print CZDS Terms & Conditions")
	requestTLDs = flag.String("request", "", "comma separated list of zones to request")
	requestAll  = flag.Bool("request-all", false, "request all available zones")
	status      = flag.Bool("status", false, "print status of zones")
	extendTLDs  = flag.String("extend", "", "comma separated list of zones to request extensions")
	extendAll   = flag.Bool("extend-all", false, "extend all possible zones")
	exclude     = flag.String("exclude", "", "comma separated list of zones to exclude from request-all or extend-all")
	cancelTLDs  = flag.String("cancel", "", "comma separated list of zones to cancel outstanding requests for")
	showVersion = flag.Bool("version", false, "print version and exit")
)

var (
	version = "unknown"
	client  *czds.Client
)

func v(format string, v ...interface{}) {
	if *verbose {
		log.Printf(format, v...)
	}
}

func checkFlags() {
	flag.Parse()
	if *showVersion {
		fmt.Printf("Version: %s\n", version)
		os.Exit(0)
	}
	flagError := false
	if len(*username) == 0 {
		log.Printf("must pass username")
		flagError = true
	}
	if len(*password) == 0 && len(*passin) == 0 {
		log.Printf("must pass either 'password' or 'passin'")
		flagError = true
	}
	if flagError {
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func main() {
	checkFlags()

	p := *password
	if len(p) == 0 {
		pass, err := czds.Getpass(*passin)
		if err != nil {
			log.Fatal("Unable to get password from user: ", err)
		}
		p = pass
	}

	doRequest := (*requestAll || len(*requestTLDs) > 0)
	doExtend := (*extendAll || len(*extendTLDs) > 0)
	doCancel := len(*extendTLDs) > 0
	if !*printTerms && !*status && !(doRequest || doExtend) && !doCancel {
		log.Fatal("Nothing to do!")
	}

	excludeList := strings.Split(*exclude, ",")

	client = czds.NewClient(*username, p)
	if *verbose {
		client.SetLogger(log.Default())
	}

	// validate credentials
	v("Authenticating to %s", client.AuthURL)
	err := client.Authenticate()
	if err != nil {
		log.Fatal(err)
	}

	// print terms
	if *printTerms {
		terms, err := client.GetTerms()
		if err != nil {
			log.Fatal(err)
		}
		v("Terms Version %s", terms.Version)
		fmt.Println("Terms and Conditions:")
		fmt.Println(terms.Content)
	}

	// print status
	if *status {
		allTLDStatus, err := client.GetTLDStatus()
		if err != nil {
			log.Fatal(err)
		}
		for _, tldStatus := range allTLDStatus {
			printTLDStatus(tldStatus)
		}
	}

	// request
	if doRequest {
		if len(*reason) == 0 {
			log.Fatal("Must pass a reason to request TLDs")
		}
		var requestedTLDs []string
		if *requestAll {
			v("Requesting all TLDs")
			requestedTLDs, err = client.RequestAllTLDsExcept(*reason, excludeList)
		} else {
			tlds := strings.Split(*requestTLDs, ",")
			v("Requesting %v", tlds)
			err = client.RequestTLDs(tlds, *reason)
			requestedTLDs = tlds
		}

		if err != nil {
			log.Fatal(err)
		}
		if len(requestedTLDs) > 0 {
			fmt.Printf("Requested: %v\n", requestedTLDs)
		}
	}
	// extend
	if doExtend {
		var extendedTLDs []string
		if *extendAll {
			v("Requesting extension for all TLDs")
			extendedTLDs, err = client.ExtendAllTLDsExcept(excludeList)
		} else {
			tlds := strings.Split(*extendTLDs, ",")
			for _, tld := range tlds {
				v("Requesting extension %v", tld)
				err = client.ExtendTLD(tld)
				if err != nil {
					// stop on first error
					break
				}
			}
			extendedTLDs = tlds
		}

		if err != nil {
			log.Fatal(err)
		}
		if len(extendedTLDs) > 0 {
			fmt.Printf("Extended: %v\n", extendedTLDs)
		}
	}
	// cancel
	if doCancel {
		tlds := strings.Split(*cancelTLDs, ",")
		for _, tld := range tlds {
			v("Requesting cancellation %v", tld)
			err = cancelRequest(tld)
			if err != nil {
				// stop on first error
				break
			}
		}
		if err != nil {
			log.Fatal(err)
		}
		if len(tlds) > 0 {
			fmt.Printf("Canceled: %v\n", tlds)
		}
	}
}

func printTLDStatus(tldStatus czds.TLDStatus) {
	fmt.Printf("%s\t%s\n", tldStatus.TLD, tldStatus.CurrentStatus)
}

func cancelRequest(zone string) error {
	zoneID, err := client.GetZoneRequestID(zone)
	if err != nil {
		return err
	}
	cancelRequest := &czds.CancelRequestSubmission{
		RequestID: zoneID,
		TLDName:   zone,
	}
	_, err = client.CancelRequest(cancelRequest)
	if err != nil {
		return err
	}
	return nil
}
