// vim: ts=4:sw=4:sts=4
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type Repository struct {
	Target   string        `yaml:"target"`
	Packages yaml.MapSlice `yaml:"packages"`
}

type GitPackage struct {
	GitURL string `yaml:"git"`
	Subdir string `yaml:"dir"`
	Ref    string `yaml:"ref"`
}

func readYAML(file string, object interface{}) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, object)
}

func writeYAML(file string, object interface{}) error {
	data, err := yaml.Marshal(object)
	if err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func main() {
	var res int = 0
	defer func() { os.Exit(res) }()

	targetOpt := flag.String("t", "", "FPM target")
	sudoOpt := flag.Bool("sudo", false, "Use sudo in fpmbuild")
	datadirOpt := flag.String("datadir", "", "Data directory")
	flag.Parse()
	args := flag.Args()

	for _, arg := range args {
		res += run(arg, *targetOpt, *sudoOpt, *datadirOpt)
	}
}
func run(repofname string, target string, sudo bool, datadir string) (res int) {
	var repo Repository
	var repodir string
	var repoyaml string

	st, st_err := os.Stat(repofname)
	if st_err != nil && (!os.IsNotExist(st_err) || datadir == "") {
		log.Println(st_err)
		res = 1
		return
	} else if datadir != "" && st_err != nil {
		repodir = filepath.Join(datadir, repofname)
		repoyaml = filepath.Join(repodir, "_repo.yaml")
	} else if st.IsDir() {
		repoext := filepath.Ext(repofname)
		repodir = repofname[:len(repofname)-len(repoext)]
		if datadir != "" {
			repodir = filepath.Join(datadir, filepath.Base(repodir))
		}
		repoyaml = filepath.Join(repodir+".src", "_repo.yaml")
	} else {
		repoext := filepath.Ext(repofname)
		repodir = repofname[:len(repofname)-len(repoext)]
		repoyaml = repofname
		if datadir != "" {
			repodir = filepath.Join(datadir, filepath.Base(repodir))
		}
	}

	err := readYAML(repoyaml, &repo)
	if err != nil {
		log.Println(err)
		res = 1
		return
	}
	if target == "" {
		target = repo.Target
	}

	repotargetdir := fmt.Sprintf("%s.%s", repodir, target)
	reposrcdir := fmt.Sprintf("%s.src", repodir)
	repopkgdir := fmt.Sprintf("%s.%s", repotargetdir, time.Now().Format("20060102-150405"))
	log.Printf("Starting fpmbot2...")
	log.Printf("Building packages from %s", reposrcdir)
	log.Printf("Writing packages to %s", repopkgdir)

	repoprevdir, err := os.Readlink(repotargetdir)
	if err != nil && !os.IsNotExist(err) {
		log.Println(err)
		res = 1
		return
	} else if err == nil {
		repoprevdir = filepath.Join(filepath.Dir(repotargetdir), repoprevdir)
		log.Printf("Previous build in %s", repoprevdir)
	} else {
		repoprevdir = ""
		log.Println("First build")
	}

	err = os.MkdirAll(reposrcdir, 0777)
	if err != nil {
		log.Println(err)
		res = 1
		return
	}

	for _, item := range repo.Packages {
		name := item.Key.(string)
		srcdir := filepath.Join(reposrcdir, name)
		pkgdir := filepath.Join(repopkgdir, name)
		prevdir := ""
		if repoprevdir != "" {
			prevdir = filepath.Join(repoprevdir, name)
		}

		err := os.MkdirAll(pkgdir, 0777)
		if err != nil {
			log.Println(err)
			res += 1
			continue
		}

		log.Printf("Package %s", name)
		if item.Value != nil {
			err = writeYAML(srcdir+".yaml", item.Value)
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}
		}

		var gitpkg GitPackage
		err = readYAML(srcdir+".yaml", &gitpkg)
		if err != nil {
			log.Println(err)
			res += 1
			continue
		}

		err = os.MkdirAll(srcdir, 0777)
		if err != nil {
			log.Println(err)
			res += 1
			continue
		}

		dirty := true

		if gitpkg.GitURL != "" {

			if _, e := os.Stat(filepath.Join(srcdir, ".git")); os.IsNotExist(e) {
				log.Printf("git init %s", srcdir)
				cmd := exec.Command("git", "init", srcdir)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err := cmd.Run()
				if err != nil {
					log.Println(err)
					res += 1
					continue
				}
			}

			log.Printf("git config remote.origin.url %s", gitpkg.GitURL)
			cmd := exec.Command("git", "config", "remote.origin.url", gitpkg.GitURL)
			cmd.Dir = srcdir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

			log.Printf("git -c core.bare=true fetch -f origin +refs/*:refs/* HEAD")
			cmd = exec.Command("git", "-c", "core.bare=true", "fetch", "-f", "origin", "+refs/*:refs/*", "HEAD")
			cmd.Dir = srcdir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				log.Println(err)
				res += 1
			}

			ref := "FETCH_HEAD"
			if gitpkg.Ref != "" && gitpkg.Ref != "HEAD" {
				ref = gitpkg.Ref
			}
			log.Printf("git reset --hard %s --", ref)
			cmd = exec.Command("git", "reset", "--hard", ref, "--")
			cmd.Dir = srcdir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

			log.Printf("git submodule update --init --force --checkout --recursive")
			cmd = exec.Command("git", "submodule", "update", "--init", "--force", "--checkout", "--recursive")
			cmd.Dir = srcdir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

			revabs, err := GitRevParseHead(srcdir)
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

			revokfile, err := ioutil.ReadFile(srcdir + ".ok")
			if err != nil && !os.IsNotExist(err) {
				log.Println(err)
				res += 1
				continue
			}

			if err == nil && string(revokfile) == revabs && prevdir != "" {
				dirty = false
				log.Printf("Package already at revision %s", revabs)
			}
		}

		if !dirty {

			log.Printf("Not rebuilding, taking packages at %s", prevdir)

			err := LinkRecursive(prevdir, pkgdir)
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

		} else {

			pkgdirabs, err := filepath.Abs(pkgdir)
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

			srcsubdir := srcdir
			if gitpkg.Subdir != "" {
				srcsubdir = filepath.Join(srcsubdir, gitpkg.Subdir)
			}

			backdir, err := filepath.Rel(srcsubdir, reposrcdir)
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

			args := []string{"-config", filepath.Join(backdir, name+".yaml"), "-f", "-o", pkgdirabs, "-t", target}
			if sudo {
				args = append([]string{"-sudo"}, args...)
			}
			log.Printf("fpmbuild %s", strings.Join(args, " "))
			cmd := exec.Command("fpmbuild", args...)
			cmd.Dir = srcsubdir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

		}

		if gitpkg.GitURL != "" {

			revabs, err := GitRevParseHead(srcdir)
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

			err = ioutil.WriteFile(srcdir+".ok", []byte(revabs), 0666)
			if err != nil {
				log.Println(err)
				res += 1
				continue
			}

			log.Printf("Build successful at revision %s", revabs)
		} else {
			log.Printf("Build successful")
		}
	}

	log.Println("Package build successful, generating metadata")

	log.Printf("fprepo-%s %s", target, filepath.Base(repodir))
	cmd := exec.Command("fprepo-"+target, filepath.Base(repodir))
	cmd.Dir = repopkgdir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Println(err)
		res += 1
		return
	}

	log.Println("Switching over to the new repository")

	err = os.Remove(repotargetdir + ".new")
	if err != nil && !os.IsNotExist(err) {
		log.Println(err)
		res += 1
		return
	}

	err = os.Symlink(filepath.Base(repopkgdir), repotargetdir+".new")
	if err != nil {
		log.Println(err)
		res += 1
		return
	}

	err = os.Rename(repotargetdir+".new", repotargetdir)
	if err != nil {
		log.Println(err)
		res += 1
		return
	}

	log.Printf("%s -> %s", repotargetdir, filepath.Base(repopkgdir))

	log.Printf("fpprunerepo -f %s", repotargetdir)
	cmd = exec.Command("fpprunerepo", "-f", repotargetdir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Println(err)
	}
	return 0
}

func GitRevParseHead(dir string) (string, error) {
	var revabs bytes.Buffer
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Stderr = os.Stderr
	cmd.Stdout = &revabs
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return string(revabs.Bytes()), nil
}

func LinkRecursive(from, to string) error {
	st, err := os.Lstat(from)
	if err != nil {
		return err
	}

	if st.IsDir() {
		err := os.MkdirAll(to, st.Mode())
		if err != nil {
			return err
		}
		d, err := os.Open(from)
		if err != nil {
			return err
		}
		defer d.Close()
		names, err := d.Readdirnames(-1)
		if err != nil {
			return err
		}
		for _, n := range names {
			err := LinkRecursive(filepath.Join(from, n), filepath.Join(to, n))
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		return os.Link(from, to)
	}
}
