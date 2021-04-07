package main

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Page struct {
	Title string
	Body  []byte
}

func main() {

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	ws := make(chan int)

	go setupWebServer(ws, port)

	x := <-ws

	fmt.Println(x, x)
}

var (

	templates  = template.Must(template.ParseFiles("edit.html", "view.html"))
	validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")
)

func setupWebServer(c chan int, port string) {

	http.HandleFunc("/view/", viewHandler)
	http.HandleFunc("/edit/", editHandler)
	http.HandleFunc("/save/", saveHandler)

	http.HandleFunc("/execCommand/", execCmdHandler)
	http.HandleFunc("/getDrives/", getDrives)
	http.HandleFunc("/getDirectories/", getDirectories)

	log.Printf("Listening on port %s", port)
	log.Printf("Open http://localhost:%s in the browser", port)

	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal(err)
	}

	c <- 200
}

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {

	m := validPath.FindStringSubmatch(r.URL.Path)

	if m == nil {

		http.NotFound(w, r)

		return "", errors.New("invalid Page Title")

	}

	return m[2], nil // The title is the second subexpression.

}

func getDirectories(writer http.ResponseWriter, request *http.Request) {

	dirs, ok := request.URL.Query()["dir"]

	if !ok || len(dirs) < 1 {

		log.Println("Url Param 'dir' is missing")

		return

	}

	var fsItems = getDirectoryInfo(dirs, 1000, 1)

	title := "/view/"

	p, err := loadPage(title)

	if err != nil {

		fmt.Fprintln(writer, err)

	} else {

		t, err := template.ParseFiles("view.html")

		if err!=nil {

			fmt.Fprintln(writer, err)

		} else {

			for e := fsItems.Front(); e != nil; e = e.Next() {

				err = t.Execute(writer, p)

				if err!=nil{

					fmt.Fprintln(writer, err)

				}
			}
		}
	}
}

func getDrives(writer http.ResponseWriter, request *http.Request) {

	if request.URL.Path == "/getDrives/" {

		var drvs = getOSDrives()
		for _, drv := range drvs {
			fmt.Fprintln(writer, drv)
		}
	}
}

func execCmdHandler(writer http.ResponseWriter, request *http.Request) {

	cmds, ok := request.URL.Query()["cmd"]
	if !ok || len(cmds) < 1 {
		fmt.Fprintln(writer, fmt.Sprintf("Execute Command Failed with Error: %s.\n", "url Param 'cmd' is missing"), nil)
	}

	var stdError string
	var stdOut string
	var err error
	for cmd := range cmds {
		err, stdOut, stdError = executeShellCommand(string(cmd))
		if err!=nil {
			fmt.Fprintln(writer, fmt.Sprintf("Execute Command Failed with Error: %s : %s.\n", stdError, err), nil)
		}
		fmt.Fprintln(writer, fmt.Sprintf("Executed Command Successfully: %s.\n", stdOut), nil)
	}

}

func viewHandler(w http.ResponseWriter, r *http.Request) {

	title := r.URL.Path[len("/view/"):]

	p, _ := loadPage(title)

	t, _ := template.ParseFiles("view.html")

	t.Execute(w, p)

}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {

	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func editHandler(w http.ResponseWriter, r *http.Request){

}

func saveHandler(w http.ResponseWriter, r *http.Request){

	title := r.URL.Path[len("/save/"):]

	body := r.FormValue("body")

	p := &Page{Title: title, Body: []byte(body)}

	err := p.save()

	if err != nil {

		http.Error(w, err.Error(), http.StatusInternalServerError)

		return

	}

	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func enableCors(w *http.ResponseWriter) {

	(*w).Header().Set("Access-Control-Allow-Origin", "*")

}

func initialize(w http.ResponseWriter, r *http.Request) (bool) {

	enableCors(&w)

	if r.Method != "GET" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return false
	}

	return true
}

func getOSDrives() (r []string) {

	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		f, err := os.Open(string(drive) + ":\\")
		if err == nil {
			r = append(r, string(drive))
			f.Close()
		}

	}
	return
}

func getDirectoryInfo(dirs []string, maxDirs int, depth int) *list.List {

	fileList := list.New()
	var numDirsMax = 0

	for _, dir := range dirs {

		fsItems, err := ioutil.ReadDir(string(dir))
		if err != nil {
			log.Fatal(err)
		}

		for _, fsItem := range fsItems {

			fqn := string(dir) + fsItem.Name()

			if numDirsMax > maxDirs {

				break

			} else if fsItem.IsDir() && depth > 1 {

				var l = ReadDirectory(fqn, depth)

				for e := l.Front(); e != nil; e = e.Next() {

					fdn := fmt.Sprintf("%s", e.Value)

					fileList.PushBack(fdn)

				}

			} else {

				fileList.PushBack(fqn)
			}
			numDirsMax++
		}

	}
	return fileList
}

func ReadDirectory(d string, depth int) *list.List {

	var files []string

	root := d

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {

		if strings.Count(path, "\\") > depth {

			return nil

		} else {

			files = append(files, path)

			return nil
		}

	})

	if err != nil {

		panic(err)

	}

	fileList := list.New()

	for _, file := range files {

		fileList.PushBack(file)

	}

	return fileList
}

func loadPage(title string) (*Page, error) {

	filename := title + ".html"

	body, err := ioutil.ReadFile(filename)

	if err != nil {

		return nil, err

	}

	return &Page{Title: title, Body: body}, nil
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

const ShellToUse = "bash"

func executeShellCommand(command string) (error, string, string) {

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(ShellToUse, "-c", command)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return err, stdout.String(), stderr.String()
}