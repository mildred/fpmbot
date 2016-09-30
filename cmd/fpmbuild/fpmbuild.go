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

	"gopkg.in/yaml.v2"
)

func main() {
	var res int = 0
	defer func() { os.Exit(res) }()

	repoYAMLFile := flag.String("config", "", "YAML configuration")
	sudoFlag := flag.Bool("sudo", false, "Use sudo to invoke docker")
	target := flag.String("t", "", "FPM target")
	outPath := flag.String("o", ".", "Output (directory or file)")
	forceFPM := flag.Bool("f", true, "Force writing package (fpm option -f)")
	flag.Parse()
	args := flag.Args()
	dockerSudo = *sudoFlag
	fmt.Println("fpmbuild starting...")

	var configFile, packageFile FPMBuildFile

	if *repoYAMLFile != "" {
		err := readYAML(*repoYAMLFile, &configFile)
		if err != nil {
			log.Println(err)
		}
	}
	err := readYAML(".fpmbuild.yaml", &packageFile)
	if err != nil {
		log.Println(err)
	}

	fpmbuild := merge(merge(configFile, packageFile), defaultFile)

	if len(args) > 0 {
		err := os.Chdir(args[0])
		if err != nil {
			log.Println(err)
			res = 1
			return
		}
	}

	if fpmbuild.Clean != "" {
		args := []string{"clean"}
		args = append(args, fpmbuild.Clean)
		log.Printf("git %s", strings.Join(args, " "))
		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Println(err)
			res = 1
			return
		}
	}

	var env Environment
	if fpmbuild.Environment.Docker != nil {
		log.Println("Use Docker")
		env = fpmbuild.Environment.Docker
	} else {
		log.Println("Use Host system")
		env = &DefaultEnvironment{}
	}

	command := fpmbuild.Build.Command()
	log.Println(strings.Join(command, " "))
	err = env.Execute(command)
	if err != nil {
		log.Println(err)
		res = 1
		return
	}

	opts := ""
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	} else {
		base := filepath.Base(path)
		if base != "" && base != "." {
			opts += " --name=" + shellEscape(base)
		}
	}

	cmd := exec.Command("git", "rev-parse")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if cmd.Run() == nil {

		// Compute staging area hash

		dirtymark := ".dirty"
		environ := os.Environ()
		environ = append(environ, "GIT_INDEX_FILE=.git/index-fpm-dirty")
		os.Remove(".git/index-fpm-dirty")
		defer func() { os.Remove(".git/index-fpm-dirty") }()

		cmd := exec.Command("git", "add", "-u")
		cmd.Env = environ
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Println(err)
		} else {
			cmd := exec.Command("git", "reset")
			cmd.Env = environ
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				log.Println(err)
			}
		}
		if err == nil {
			var hash bytes.Buffer
			cmd := exec.Command("git", "write-tree")
			cmd.Env = environ
			cmd.Stdout = &hash
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				log.Println(err)
			} else {
				dirtymark += "." + string(hash.Bytes()[0:7])
			}
		}

		// Describe Git HEAD

		var buf bytes.Buffer
		cmd = exec.Command("git", "describe", "--dirty="+dirtymark)
		cmd.Stdout = &buf
		if cmd.Run() == nil {
			ver := ""
			for _, b := range buf.Bytes() {
				if ver == "" {
					if b >= '0' && b <= '9' {
						ver += string([]byte{b})
					}
				} else if (b < '0' || b > '9') &&
					(b < 'a' || b > 'z') &&
					(b < 'A' || b > 'Z') {
					ver += "."
				} else {
					ver += string([]byte{b})
				}
			}
			ver = strings.TrimRight(ver, ".")
			opts += " --version=" + shellEscape(ver)

		} else {

			buf.Reset()
			cmd = exec.Command("git", "describe", "--dirty="+dirtymark, "--always", "--tags")
			cmd.Stdout = &buf
			if err := cmd.Run(); err != nil {
				log.Println(err)
			} else {
				ver := "0." + strings.Trim(string(buf.Bytes()), " \n")
				opts += " --version=" + shellEscape(ver)
			}
		}
	}

	args = []string{"-t", *target, "-p", *outPath}
	if *forceFPM {
		args = append(args, "-f")
	}
	var tempfiles []string
	defer func() {
		for _, f := range tempfiles {
			err := os.Remove(f)
			if err != nil {
				log.Println(err)
			}
		}
	}()
	for k, v := range fpmbuild.FPMHooks {
		log.Printf("fpm %s:\n  %s", k, strings.Replace(v, "\n", "\n  ", -1))
		f, err := ioutil.TempFile("", k)
		if err != nil {
			log.Println(err)
			res = 1
			return
		}
		tempfiles = append(tempfiles, f.Name())
		defer f.Close()
		_, err = f.Write([]byte(v))
		if err != nil {
			log.Println(err)
			res = 1
			return
		}
		err = os.Chmod(f.Name(), 0755)
		if err != nil {
			log.Println(err)
			res = 1
			return
		}
		args = append(args, "--"+k, f.Name())
	}
	args = append(args, fpmbuild.FPM...)
	log.Printf("fpm [%s ] %s", opts, strings.Join(args, " "))
	cmd = exec.Command("fpm", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Env = append(os.Environ(), "FPMOPTS="+opts)
	err = cmd.Run()
	if err != nil {
		log.Println(err)
		res = 1
	}
}

func shellEscape(s string) string {
	s = strings.Replace(s, `'`, `'"'"'`, -1)
	return `'` + s + `'`
}

func readYAML(filename string, obj interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(bytes, obj)
}

func mergeString(file1, file2 string) string {
	if file1 != "" {
		return file1
	} else {
		return file2
	}
}

func mergeStrings(file1, file2 []string) []string {
	if file1 != nil {
		return file1
	} else {
		return file2
	}
}

func mergeStringMap(file1, file2 map[string]string) map[string]string {
	res := map[string]string{}
	for k, v := range file2 {
		if v == "" {
			delete(res, k)
		} else {
			res[k] = v
		}
	}
	for k, v := range file1 {
		if v == "" {
			delete(res, k)
		} else {
			res[k] = v
		}
	}
	return res
}

func merge(file1, file2 FPMBuildFile) (res FPMBuildFile) {
	res.Build.Prepare = mergeString(file1.Build.Prepare, file2.Build.Prepare)
	res.Build.FPMGen = mergeString(file1.Build.FPMGen, file2.Build.FPMGen)
	res.Build.Build = mergeString(file1.Build.Build, file2.Build.Build)
	res.Build.Install = mergeString(file1.Build.Install, file2.Build.Install)
	res.Build.Shell = mergeString(file1.Build.Shell, file2.Build.Shell)
	res.Build.Options = mergeStrings(file1.Build.Options, file2.Build.Options)
	res.Build.Arguments = mergeStrings(file1.Build.Arguments, file2.Build.Arguments)
	res.FPM = mergeStrings(file1.FPM, file2.FPM)
	res.FPMHooks = mergeStringMap(file1.FPMHooks, file2.FPMHooks)
	res.Clean = mergeString(file1.Clean, file2.Clean)

	if file1.Environment.Docker != nil {
		res.Environment = file1.Environment
	} else {
		res.Environment = file2.Environment
	}
	return res
}
