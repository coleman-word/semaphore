package util

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"

	"net/url"

	"github.com/gorilla/securecookie"
	"golang.org/x/crypto/bcrypt"
)

var Cookie *securecookie.SecureCookie
var Migration bool
var InteractiveSetup bool
var Upgrade bool
var WebHostURL *url.URL

type mySQLConfig struct {
	Hostname string `json:"host"`
	Username string `json:"user"`
	Password string `json:"pass"`
	DbName   string `json:"name"`
}

type ldapMappings struct {
	DN   string `json:"dn"`
	Mail string `json:"mail"`
	Uid  string `json:"uid"`
	CN   string `json:"cn"`
}

type configType struct {
	MySQL mySQLConfig `json:"mysql"`
	// Format `:port_num` eg, :3000
	Port string `json:"port"`

	// semaphore stores projects here
	TmpPath string `json:"tmp_path"`

	// cookie hashing & encryption
	CookieHash       string `json:"cookie_hash"`
	CookieEncryption string `json:"cookie_encryption"`

	// email alerting
	EmailAlert  bool   `json:"email_alert"`
	EmailSender string `json:"email_sender"`
	EmailHost   string `json:"email_host"`
	EmailPort   string `json:"email_port"`

	// web host
	WebHost string `json:"web_host"`

	// ldap settings
	LdapEnable       bool         `json:"ldap_enable"`
	LdapBindDN       string       `json:"ldap_binddn"`
	LdapBindPassword string       `json:"ldap_bindpassword"`
	LdapServer       string       `json:"ldap_server"`
	LdapNeedTLS      bool         `json:"ldap_needtls"`
	LdapSearchDN     string       `json:"ldap_searchdn"`
	LdapSearchFilter string       `json:"ldap_searchfilter"`
	LdapMappings     ldapMappings `json:"ldap_mappings"`

	// telegram alerting
	TelegramAlert bool   `json:"telegram_alert"`
	TelegramChat  string `json:"telegram_chat"`
	TelegramToken string `json:"telegram_token"`

	// task concurrency
	ConcurrencyMode  string `json:"concurrency_mode"`
	MaxParallelTasks int    `json:"max_parallel_tasks"`
}

var Config *configType

func NewConfig() *configType {
	return &configType{}
}

func init() {
	flag.BoolVar(&InteractiveSetup, "setup", false, "perform interactive setup")
	flag.BoolVar(&Migration, "migrate", false, "execute migrations")
	flag.BoolVar(&Upgrade, "upgrade", false, "upgrade semaphore")
	path := flag.String("config", "", "config path")

	var pwd string
	flag.StringVar(&pwd, "hash", "", "generate hash of given password")

	var printConfig bool
	flag.BoolVar(&printConfig, "printConfig", false, "print example configuration")

	flag.Parse()

	if printConfig {
		cfg := &configType{
			MySQL: mySQLConfig{
				Hostname: "127.0.0.1:3306",
				Username: "root",
				DbName:   "semaphore",
			},
			Port:    ":3000",
			TmpPath: "/tmp/semaphore",
		}
		cfg.GenerateCookieSecrets()

		b, _ := json.MarshalIndent(cfg, "", "\t")
		fmt.Println(string(b))

		os.Exit(0)
	}

	if len(pwd) > 0 {
		password, _ := bcrypt.GenerateFromPassword([]byte(pwd), 11)
		fmt.Println("Generated password: ", string(password))

		os.Exit(0)
	}

	if path != nil && len(*path) > 0 {
		// load
		file, err := os.Open(*path)
		if err != nil {
			panic(err)
		}

		if err := json.NewDecoder(file).Decode(&Config); err != nil {
			fmt.Println("Could not decode configuration!")
			panic(err)
		}
	} else {
		configFile, err := Asset("config.json")
		if err != nil {
			fmt.Println("Cannot Find configuration! Use -c parameter to point to a JSON file generated by -setup.\n\n Hint: have you run `-setup` ?")
			os.Exit(1)
		}

		if err := json.Unmarshal(configFile, &Config); err != nil {
			fmt.Println("Could not decode configuration!")
			panic(err)
		}
	}

	if len(os.Getenv("PORT")) > 0 {
		Config.Port = ":" + os.Getenv("PORT")
	}
	if len(Config.Port) == 0 {
		Config.Port = ":3000"
	}

	if len(Config.TmpPath) == 0 {
		Config.TmpPath = "/tmp/semaphore"
	}

	if Config.MaxParallelTasks < 1 {
		Config.MaxParallelTasks = 10
	}

	var encryption []byte
	encryption = nil

	hash, _ := base64.StdEncoding.DecodeString(Config.CookieHash)
	if len(Config.CookieEncryption) > 0 {
		encryption, _ = base64.StdEncoding.DecodeString(Config.CookieEncryption)
	}

	Cookie = securecookie.New(hash, encryption)
	WebHostURL, _ = url.Parse(Config.WebHost)
	if len(WebHostURL.String()) == 0 {
		WebHostURL = nil
	}
}

func (conf *configType) GenerateCookieSecrets() {
	hash := securecookie.GenerateRandomKey(32)
	encryption := securecookie.GenerateRandomKey(32)

	conf.CookieHash = base64.StdEncoding.EncodeToString(hash)
	conf.CookieEncryption = base64.StdEncoding.EncodeToString(encryption)
}

func (conf *configType) Scan() {
	fmt.Print(" > DB Hostname (default 127.0.0.1:3306): ")
	fmt.Scanln(&conf.MySQL.Hostname)
	if len(conf.MySQL.Hostname) == 0 {
		conf.MySQL.Hostname = "127.0.0.1:3306"
	}

	fmt.Print(" > DB User (default root): ")
	fmt.Scanln(&conf.MySQL.Username)
	if len(conf.MySQL.Username) == 0 {
		conf.MySQL.Username = "root"
	}

	fmt.Print(" > DB Password: ")
	fmt.Scanln(&conf.MySQL.Password)

	fmt.Print(" > DB Name (default semaphore): ")
	fmt.Scanln(&conf.MySQL.DbName)
	if len(conf.MySQL.DbName) == 0 {
		conf.MySQL.DbName = "semaphore"
	}

	fmt.Print(" > Playbook path: ")
	fmt.Scanln(&conf.TmpPath)

	if len(conf.TmpPath) == 0 {
		conf.TmpPath = "/tmp/semaphore"
	}
	conf.TmpPath = path.Clean(conf.TmpPath)

	fmt.Print(" > Web root URL (optional, example http://localhost:8010/): ")
	fmt.Scanln(&conf.WebHost)

	var EmailAlertAnswer string
	fmt.Print(" > Enable email alerts (y/n, default n): ")
	fmt.Scanln(&EmailAlertAnswer)
	if EmailAlertAnswer == "yes" || EmailAlertAnswer == "y" {

		conf.EmailAlert = true

		fmt.Print(" > Mail server host (default localhost): ")
		fmt.Scanln(&conf.EmailHost)

		if len(conf.EmailHost) == 0 {
			conf.EmailHost = "localhost"
		}

		fmt.Print(" > Mail server port (default 25): ")
		fmt.Scanln(&conf.EmailPort)

		if len(conf.EmailPort) == 0 {
			conf.EmailPort = "25"
		}

		fmt.Print(" > Mail sender address (default semaphore@localhost): ")
		fmt.Scanln(&conf.EmailSender)

		if len(conf.EmailSender) == 0 {
			conf.EmailSender = "semaphore@localhost"
		}

	} else {
		conf.EmailAlert = false
	}

	var TelegramAlertAnswer string
	fmt.Print(" > Enable telegram alerts (y/n, default n): ")
	fmt.Scanln(&TelegramAlertAnswer)
	if TelegramAlertAnswer == "yes" || TelegramAlertAnswer == "y" {

		conf.TelegramAlert = true

		fmt.Print(" > Telegram bot token (you can get it from @BotFather) (default ''): ")
		fmt.Scanln(&conf.TelegramToken)

		if len(conf.TelegramToken) == 0 {
			conf.TelegramToken = ""
		}

		fmt.Print(" > Telegram chat ID (default ''): ")
		fmt.Scanln(&conf.TelegramChat)

		if len(conf.TelegramChat) == 0 {
			conf.TelegramChat = ""
		}

	} else {
		conf.TelegramAlert = false
	}

	var LdapAnswer string
	fmt.Print(" > Enable LDAP authentication (y/n, default n): ")
	fmt.Scanln(&LdapAnswer)
	if LdapAnswer == "yes" || LdapAnswer == "y" {

		conf.LdapEnable = true

		fmt.Print(" > LDAP server host (default localhost:389): ")
		fmt.Scanln(&conf.LdapServer)

		if len(conf.LdapServer) == 0 {
			conf.LdapServer = "localhost:389"
		}

		var LdapTLSAnswer string
		fmt.Print(" > Enable LDAP TLS connection (y/n, default n): ")
		fmt.Scanln(&LdapTLSAnswer)
		if LdapTLSAnswer == "yes" || LdapTLSAnswer == "y" {
			conf.LdapNeedTLS = true
		} else {
			conf.LdapNeedTLS = false
		}

		fmt.Print(" > LDAP DN for bind (default cn=user,ou=users,dc=example): ")
		fmt.Scanln(&conf.LdapBindDN)

		if len(conf.LdapBindDN) == 0 {
			conf.LdapBindDN = "cn=user,ou=users,dc=example"
		}

		fmt.Print(" > Password for LDAP bind user (default pa55w0rd): ")
		fmt.Scanln(&conf.LdapBindPassword)

		if len(conf.LdapBindPassword) == 0 {
			conf.LdapBindPassword = "pa55w0rd"
		}

		fmt.Print(" > LDAP DN for user search (default ou=users,dc=example): ")
		fmt.Scanln(&conf.LdapSearchDN)

		if len(conf.LdapSearchDN) == 0 {
			conf.LdapSearchDN = "ou=users,dc=example"
		}

		fmt.Print(" > LDAP search filter (default (uid=" + "%" + "s)): ")
		fmt.Scanln(&conf.LdapSearchFilter)

		if len(conf.LdapSearchFilter) == 0 {
			conf.LdapSearchFilter = "(uid=%s)"
		}

		fmt.Print(" > LDAP mapping for DN field (default dn): ")
		fmt.Scanln(&conf.LdapMappings.DN)

		if len(conf.LdapMappings.DN) == 0 {
			conf.LdapMappings.DN = "dn"
		}

		fmt.Print(" > LDAP mapping for username field (default uid): ")
		fmt.Scanln(&conf.LdapMappings.Uid)

		if len(conf.LdapMappings.Uid) == 0 {
			conf.LdapMappings.Uid = "uid"
		}

		fmt.Print(" > LDAP mapping for full name field (default cn): ")
		fmt.Scanln(&conf.LdapMappings.CN)

		if len(conf.LdapMappings.CN) == 0 {
			conf.LdapMappings.CN = "cn"
		}

		fmt.Print(" > LDAP mapping for email field (default mail): ")
		fmt.Scanln(&conf.LdapMappings.Mail)

		if len(conf.LdapMappings.Mail) == 0 {
			conf.LdapMappings.Mail = "mail"
		}

	} else {
		conf.LdapEnable = false
	}

}
