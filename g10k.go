package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	//"net/http"
	"regexp"

	"code.google.com/p/gcfg"
	//"gopkg.in/libgit2/git2go.v22"
	"github.com/libgit2/git2go"
)

var debug bool
var verbose bool
var mainCfgSection = make(map[string]string)
var gitCfgSection = make(map[string]string)
var forgeCfgSection = make(map[string]string)
var cacheDir string
var moduleDir string

// ConfigSettings contains the key value pairs from the config file
type ConfigSettings struct {
	main  map[string]string
	git   map[string]string
	forge map[string]string
}

// Debugf is a helper function for debug logging if mainCfgSection["debug"] is set
func Debugf(s string) {
	if debug != false {
		log.Print("DEBUG " + fmt.Sprint(s))
	}
}

// Verbosef is a helper function for debug logging if mainCfgSection["debug"] is set
func Verbosef(s string) {
	if debug != false || verbose != false {
		log.Print(fmt.Sprint(s))
	}
}

// readConfigfile creates the CfgSections structs from the config file
func readConfigfile(configFile string) ConfigSettings {
	var cfg = &struct {
		Main struct {
			gcfg.Idxer
			Vals map[gcfg.Idx]*string
		}
		Git struct {
			gcfg.Idxer
			Vals map[gcfg.Idx]*string
		}
		Forge struct {
			gcfg.Idxer
			Vals map[gcfg.Idx]*string
		}
	}{}

	if err := gcfg.ReadFileInto(cfg, configFile); err != nil {
		log.Print("There was an error parsing the configfile "+configFile+": ", err)
	}

	cfgMain := &cfg.Main
	Debugf("Found main config settings:")
	for _, n := range cfgMain.Names() {
		mainCfgSection[n] = *cfgMain.Vals[cfgMain.Idx(n)]
		Debugf(n + " = " + *cfgMain.Vals[cfgMain.Idx(n)])
	}

	// check if moduledir exists
	if moduledirTmp, ok := mainCfgSection["moduledir"]; ok {
		moduleDir = moduledirTmp
		if _, err := os.Stat(moduledirTmp); os.IsNotExist(err) {
			log.Printf("trying to create moduledir '%s'", moduledirTmp)
			os.Mkdir(moduledirTmp, 0777)
		}
	} else {
		// TODO make moduledir optional
		log.Print("moduledir config setting missing! Exiting!")
		os.Exit(1)
	}

	if !strings.HasSuffix(moduleDir, "/") {
		moduleDir = moduleDir + "/"
	}

	Debugf("Using as moduledir: " + moduleDir)

	// check if cachedir exists
	if cacheDirTmp, ok := mainCfgSection["cachedir"]; ok {
		cacheDir = cacheDirTmp
		if _, err := os.Stat(cacheDirTmp); os.IsNotExist(err) {
			log.Printf("trying to create cachedir '%s'", cacheDirTmp)
			os.Mkdir(cacheDirTmp, 0777)
		}
	} else {
		// TODO make cachedir optional
		log.Print("cachedir config setting missing! Exiting!")
		os.Exit(1)
	}

	if !strings.HasSuffix(cacheDir, "/") {
		cacheDir = cacheDir + "/"
	}

	Debugf("Using as cachedir: " + cacheDir)

	cfgGit := &cfg.Git
	// Names(): iterate over variables with undefined order and case
	Debugf("Found git config settings:")
	for _, n := range cfgGit.Names() {
		Debugf(n + " = " + *cfgGit.Vals[cfgGit.Idx(n)])
		gitCfgSection[n] = *cfgGit.Vals[cfgGit.Idx(n)]
	}

	cfgForge := &cfg.Forge
	// Names(): iterate over variables with undefined order and case
	Debugf("Found forge config settings:")
	for _, n := range cfgForge.Names() {
		Debugf(n + " = " + *cfgForge.Vals[cfgForge.Idx(n)])
		forgeCfgSection[n] = *cfgForge.Vals[cfgForge.Idx(n)]
	}

	return ConfigSettings{mainCfgSection, gitCfgSection, forgeCfgSection}
}

func credentialsCallback(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
	Debugf("Using as ssh_pub_key: " + mainCfgSection["ssh_pub_key"])
	Debugf("Using as ssh_priv_key: " + mainCfgSection["ssh_priv_key"])
	ret, cred := git.NewCredSshKeyFromAgent(username)
	//ret, cred := git.NewCredSshKey("git", "/home/andpaul/.ssh/andpaul.pub", "/home/andpaul/.ssh/andpaul", "")
	return git.ErrorCode(ret), &cred
}

func executeGitCommand(args []string) string {
	Debugf("Executing git " + strings.Join(args, " "))
	before := time.Now()
	out, err := exec.Command("git", args...).CombinedOutput()
	Verbosef("Executing git " + strings.Join(args, " ") + " took " + strconv.FormatFloat(time.Since(before).Seconds(), 'f', 5, 64) + "s")
	if err != nil {
		log.Print("git command failed: git "+strings.Join(args, " ")+" ", err)
	}
	return string(out)
}

func compareGitVersions(targetDir string, url string, branch string) bool {
	//localChan := make(chan string)
	//remoteChan := make(chan string)

	//go func() {
	// get localVersion
	localRepo, err := git.OpenRepository(targetDir)
	if err != nil {
		panic(err)
	}
	localHead, err := localRepo.Head()
	if err != nil {
		panic(err)
	}

	//localChan <- localVersion
	//}()

	rc := git.RemoteCollection{
		repo: localRepo,
	}

	remote, err := localRepo.Lookup("origin")

	//if localHead.Cmp(remoteHead) != 0 {
	//	return false
	//} else {
	//	return true
	//}

	//go func() {
	//		remoteChan <- remoteVersion
	//	}()
	//	return <-remoteChan != <-localChan
	return true
}

func resolveGitRepositories(repos map[string]string) {
	type empty struct{}
	sem := make(chan empty, len(repos))
	for n := range repos {
		go func(n string) {
			Debugf("Trying to resolve Git repository " + n + " with " + repos[n] + cacheDir)
			branch := "master"
			opts := strings.Split(repos[n], ", ")
			Debugf("Found opts: " + strings.Join(opts, " -- "))
			url := opts[0]

			if len(opts) > 1 {
				// https://github.com/StefanSchroeder/Golang-Regex-Tutorial/blob/master/01-chapter2.markdown
				re := regexp.MustCompile("(?P<opt>(branch|commit|tag)):(?P<value>.*)")
				n1 := re.SubexpNames()
				r2 := re.FindAllStringSubmatch(opts[1], -1)[0]

				md := map[string]string{}
				for i, n := range r2 {
					//fmt.Printf("%d. match='%s'\tname='%s'\n", i, n, n1[i])
					md[n1[i]] = n
				}

				if opt, ok := md["opt"]; ok {
					// TODO commit hash
					// TODO tag
					if opt == "branch" {
						branch = md["value"]
					}
				}
			}

			Debugf("Using branch: " + branch)
			Debugf("Using url: " + url)

			// create save directory name from Git repo name
			repoDir := strings.Replace(strings.Replace(url, "/", "_", -1), ":", "-", -1)
			targetDir := cacheDir + repoDir + "_" + branch

			doCheckout := false
			dirExists := false
			if _, err := os.Stat(targetDir); os.IsNotExist(err) {
				doCheckout = true
				dirExists = false
			} else {
				dirExists = true
				doCheckout = compareGitVersions(targetDir, url, branch)
			}

			if doCheckout {
				args := []string{}
				if dirExists {
					args = append(args, "--git-dir")
					args = append(args, targetDir+"/.git")
					args = append(args, "pull")
					executeGitCommand(args)

				} else {
					// https://golog.co/blog/article/Git2Go
					cloneOptions := &git.CloneOptions{}
					cloneOptions.CheckoutBranch = branch
					cloneOptions.Bare = true
					cloneOptions.FetchOptions = &git.FetchOptions{
						RemoteCallbacks: git.RemoteCallbacks{
							CredentialsCallback: credentialsCallback,
							CertificateCheckCallback: func(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
								return 0
							},
						},
					}

					before := time.Now()
					_, err := git.Clone(url, targetDir, cloneOptions)
					if err != nil {
						panic(err)
					}
					Verbosef("git clone for " + url + " took " + strconv.FormatFloat(time.Since(before).Seconds(), 'f', 5, 64) + "s")

				}
			} else {
				Debugf("Nothing to do for Git repository '" + n + "': remote and local version are the same")
			}
			syncToModuledir(targetDir, n)
			sem <- empty{}
		}(n)
	}
	// wait for goroutines to finish
	for i := 0; i < len(repos); i++ {
		<-sem
	}
}

func syncToModuledir(srcDir string, moduleName string) {
	targetDir := moduleDir + moduleName
	if _, err := os.Stat(targetDir); err == nil {
		errr := os.Remove(targetDir)
		if err != nil {
			log.Print("error: removing Symlink failed", errr)
		}
	}

	errc := os.Symlink(srcDir, targetDir)
	if errc != nil {
		log.Print("error: creating Symlink failed", errc)
	}
}

func main() {

	var (
		configFile  = flag.String("config", "./Puppetfile.conf", "which config file to use")
		debugFlag   = flag.Bool("debug", false, "log debug output, defaults to false")
		verboseFlag = flag.Bool("verbose", false, "log verbose output, defaults to false")
	)
	flag.Parse()

	debug = *debugFlag
	verbose = *verboseFlag
	Debugf("Using as config file:" + *configFile)
	configSettings := readConfigfile(*configFile)

	resolveGitRepositories(configSettings.git)
}
