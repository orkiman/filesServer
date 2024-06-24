package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
)

const baseDir = "/home/spot/spot-or/myPhotos"                 // Replace with your actual photos directory
const thumbnailDir = "/home/spot/spot-or/myPhotos/thumbnails" // Replace with where you want to store thumbnails

func isImage(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif"
}

func generateThumbnail(src, dest string) error {
	// Ensure the destination directory exists
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

func handleFileServer(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(baseDir, r.URL.Path)

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
			// Construct the relative thumbnail path
			thumbnailPath = filepath.Join("/thumbnails", r.URL.Path, file.Name())
			// Construct the absolute path where the thumbnail should be saved
			thumbnailFullPath := filepath.Join(thumbnailDir, r.URL.Path, file.Name())

			// Check if the thumbnail already exists
			if _, err := os.Stat(thumbnailFullPath); os.IsNotExist(err) {
				// Generate the thumbnail if it doesn't exist
				err := generateThumbnail(filePath, thumbnailFullPath)
				if err != nil {
					fmt.Printf("Error generating thumbnail: %v\n", err)
					// Clear the thumbnail path if generation fails
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
                    width: 200px; /* Set the width to 200px */
                    height: auto; /* Maintain the aspect ratio */
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
	thumbnailPath := filepath.Join(thumbnailDir, strings.TrimPrefix(r.URL.Path, "/thumbnails/"))
	http.ServeFile(w, r, thumbnailPath)
}

func main() {
	http.HandleFunc("/", handleFileServer)
	http.HandleFunc("/thumbnails/", handleThumbnails)
	fmt.Println("Server starting on port 8080...")
	http.ListenAndServe(":8080", nil)
}
