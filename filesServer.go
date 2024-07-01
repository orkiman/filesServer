package main

import (
	"fmt"
	"html/template"
	"image"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
)

const baseDir = "/home/spot/spot-or/myPhotosTest" // Replace with your actual photos directory
const photosDir = baseDir + "/photos"
const thumbnailDir = baseDir + "/thumbnails" // Replace with where you want to store thumbnails
const heicDir = baseDir + "/heic"

func isImage(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".heic"
}

func convertHeicToJpeg(src string) (string, error) {
	dest := strings.TrimSuffix(src, filepath.Ext(src)) + ".jpg"
	cmd := exec.Command("magick", "convert", src, dest)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	newHeicPath := filepath.Join(heicDir, filepath.Base(src))
	os.Rename(src, newHeicPath)
	return dest, nil
}

func adjustOrientation(img image.Image, orientation int, imgName string) image.Image {
	switch orientation {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img)
	default:
		return img
	}
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

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	orientation, err := getOrientation(src)
	if err != nil {
		fmt.Println("getOrientation return error:", err)
		return err
	}

	img = adjustOrientation(img, int(orientation), filepath.Base(src))

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
		// Handle directory listing
		err := convertAllHeicFiles(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

		data := struct {
			Files []struct {
				Name          string
				IsDir         bool
				Path          string
				IsImage       bool
				ThumbnailPath string
			}
		}{
			Files: fileInfos,
		}

		tmplFile := "galleryTemplate.html"
		t, err := template.ParseFiles(tmplFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = t.Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// Handle full-size image view using imageTemplate.html
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var currentIndex, totalImages int
	var prevPath, nextPath string

	for i, file := range files {
		if isImage(file.Name()) {
			if filepath.Base(path) == file.Name() {
				currentIndex = i
			}
			totalImages++
		}
	}

	if totalImages > 1 {
		prevIndex := (currentIndex - 1 + totalImages) % totalImages
		nextIndex := (currentIndex + 1) % totalImages
		prevPath = filepath.Join(filepath.Dir(r.URL.Path), files[prevIndex].Name())
		nextPath = filepath.Join(filepath.Dir(r.URL.Path), files[nextIndex].Name())
	}

	// Debug logs
	fmt.Printf("Current image: %s\n", r.URL.Path)
	fmt.Printf("Previous image path: %s\n", prevPath)
	fmt.Printf("Next image path: %s\n", nextPath)

	data := struct {
		ImagePath string
		PrevPath  string
		NextPath  string
	}{
		// ImagePath: "/photos/" + r.URL.Path, // Use relative URL path

		// ImagePath: path, // Use relative URL path
		// ImagePath: "photos/1.jpg", //filepath.Join("photos", r.URL.Path),
		ImagePath: filepath.Join("photos", r.URL.Path),
		PrevPath:  prevPath,
		NextPath:  nextPath,
	}

	fmt.Println("r.URL.Path", r.URL.Path)
	fmt.Println("ImagePath", data.ImagePath)
	fmt.Println("path: ", path)

	tmplFile := "imageTemplate.html"
	t, err := template.ParseFiles(tmplFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleThumbnails(w http.ResponseWriter, r *http.Request) {
	thumbnailPath := filepath.Join(thumbnailDir, strings.TrimPrefix(r.URL.Path, "/thumbnails/"))
	http.ServeFile(w, r, thumbnailPath)
}

func main() {
	os.MkdirAll(heicDir, 0755)
	os.MkdirAll(thumbnailDir, 0755)
	os.MkdirAll(photosDir, 0755)
	http.HandleFunc("/", handleFileServer)
	http.Handle("/photos/", http.StripPrefix("/photos/", http.FileServer(http.Dir(photosDir))))
	http.HandleFunc("/thumbnails/", handleThumbnails)
	fmt.Println("Server starting on port 8080...")
	http.ListenAndServe(":8080", nil)
}

func getOrientation(imagePath string) (int, error) {
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		fmt.Println("Error reading image file:", err)
		return -1, err
	}

	rawExif, err := exif.SearchAndExtractExif(imageData)
	if err != nil {
		fmt.Println("Error extracting EXIF data:", err)
		return -1, err
	}

	im := exifcommon.NewIfdMapping()
	err = exifcommon.LoadStandardIfds(im)
	if err != nil {
		fmt.Println("Error loading standard IFDs:", err)
		return -1, err
	}

	ti := exif.NewTagIndex()

	_, index, err := exif.Collect(im, ti, rawExif)
	if err != nil {
		fmt.Println("Error collecting EXIF data:", err)
		return -1, err
	}

	orientationTags, err := index.RootIfd.FindTagWithName("Orientation")
	if err == nil && len(orientationTags) > 0 {
		orientationTag := orientationTags[0]
		value, err := orientationTag.FormatFirst()
		if err == nil {
			orientationInt, err := strconv.Atoi(value)
			if err != nil {
				fmt.Println("Error converting orientation to integer:", err)
				return -1, err
			}
			return orientationInt, nil
		}
	}

	return -1, fmt.Errorf("Orientation tag not found")
}
