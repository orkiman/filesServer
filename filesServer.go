package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/disintegration/imaging"
)

const baseDir = "/home/spot/spot-or/myPhotosTest" // Replace with your actual photos directory
const photosDir = baseDir + "/photos"
const thumbnailDir = baseDir + "/thumbnails" // Replace with where you want to store thumbnails
const heicDir = baseDir + "/heic"

var handleFileServerCounter uint64 = 0

func isImage(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	fmt.Println("filename : " + filename + "  ext : " + ext)
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".heic"
}

func convertHeicToJpeg(src string) (string, error) {
	dest := strings.TrimSuffix(src, filepath.Ext(src)) + ".jpg"
	cmd := exec.Command("magick", "convert", src, dest)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	// move heic to heic folder
	newHeicPath := filepath.Join(heicDir, filepath.Base(src))
	os.Rename(src, newHeicPath)
	return dest, nil
}

func generateThumbnail(src, dest string) error {
	err := os.MkdirAll(filepath.Dir(dest), 0755)
	if err != nil {
		return err
	}

	img, err := imaging.Open(src)
	if err != nil {
		return err
	}

	thumbnail := imaging.Resize(img, 200, 0, imaging.Lanczos)
	return imaging.Save(thumbnail, dest)
}

func convertAllHeicFiles(path string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath := filepath.Join(path, file.Name())
		if strings.ToLower(filepath.Ext(filePath)) == ".heic" {
			_, err := convertHeicToJpeg(filePath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func handleFileServer(w http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&handleFileServerCounter, 1)
	fmt.Printf("handleFileServer called %d times for URL: %s\n", atomic.LoadUint64(&handleFileServerCounter), r.URL.Path)

	path := filepath.Join(photosDir, r.URL.Path)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		err := convertAllHeicFiles(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}

	files, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var fileInfos []struct {
		Name          string
		IsDir         bool
		Path          string
		IsImage       bool
		ThumbnailPath string
	}

	for _, file := range files {
		filePath := filepath.Join(path, file.Name())
		isImg := isImage(file.Name())
		thumbnailPath := ""
		if isImg {
			thumbnailPath = filepath.Join("/thumbnails", r.URL.Path, file.Name())
			thumbnailFullPath := filepath.Join(thumbnailDir, r.URL.Path, file.Name())

			if _, err := os.Stat(thumbnailFullPath); os.IsNotExist(err) {
				err := generateThumbnail(filePath, thumbnailFullPath)
				if err != nil {
					fmt.Printf("Error generating thumbnail: %v\n", err)
					thumbnailPath = ""
				}
			}
		}

		fileInfos = append(fileInfos, struct {
			Name          string
			IsDir         bool
			Path          string
			IsImage       bool
			ThumbnailPath string
		}{file.Name(), file.IsDir(), filepath.Join(r.URL.Path, file.Name()), isImg, thumbnailPath})
	}

	tmpl := `
    <html>
        <head>
            <style>
                .grid {
                    display: grid;
                    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
                    gap: 10px;
                }
                .grid-item {
                    text-align: center;
                }
                .thumbnail {
                    width: 200px;
                    height: auto;
                }
            </style>
        </head>
        <body>
            <h1>File Browser</h1>
            <div class="grid">
                {{range .}}
                    <div class="grid-item">
                        {{if .IsDir}}
                            <a href="{{.Path}}">📁 {{.Name}}/</a>
                        {{else if .IsImage}}
                            <a href="{{.Path}}">
                                <img class="thumbnail" src="{{.ThumbnailPath}}" alt="{{.Name}}">
                                <br>{{.Name}}
                            </a>
                        {{else}}
                            <a href="{{.Path}}">📄 {{.Name}}</a>
                        {{end}}
                    </div>
                {{end}}
            </div>
        </body>
    </html>
    `

	t, err := template.New("filelist").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, fileInfos)
}

func handleThumbnails(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("handle thumbnail for URL: %s\n", r.URL.Path)
	thumbnailPath := filepath.Join(thumbnailDir, strings.TrimPrefix(r.URL.Path, "/thumbnails/"))
	http.ServeFile(w, r, thumbnailPath)
}

func main() {
	os.MkdirAll(heicDir, 0755)
	os.MkdirAll(thumbnailDir, 0755)
	os.MkdirAll(photosDir, 0755)
	http.HandleFunc("/", handleFileServer)
	http.HandleFunc("/thumbnails/", handleThumbnails)
	fmt.Println("Server starting on port 8080...")
	http.ListenAndServe(":8080", nil)
}
