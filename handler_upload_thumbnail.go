package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		log.Printf("Error from upload thumbnail handler when parsing %v\n", err)
	}
	thumbnailData, header, err := r.FormFile("thumbnail")
	//fmt.Printf("thumbnailData, header %+v , %+v\n", thumbnailData, header)
	if err != nil {
		log.Printf("Error from upload thumbnail handler when extracting thumbnail and header %v", err)
	}
	defer thumbnailData.Close()
	RmediaType := header.Header.Get("Content-Type")
	//ParseMediaType(v string) (mediatype string, params map[string]string, err error)
	mediatype, _, err := mime.ParseMediaType(RmediaType)
	if err != nil {
		log.Printf("error parsing media type %v\n", err)
	}
	if !(mediatype == "image/jpeg" || mediatype == "image/png") {
		log.Printf("wrong media type \n")
		respondWithError(w, http.StatusBadRequest, "Wrong file type", fmt.Errorf("Wrong file type in Content-Type"))
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorised", err)
		return
	}

	if err != nil {
		log.Printf("error from retrieval of video from database %v\n", err)

	}

	//or extract it from the string
	fileExtension := strings.Replace(RmediaType, "image/", "", 1)
	// create random file name to make sure thumbnails aren't cached
	nameBytes := make([]byte, 32)
	_, err = rand.Read(nameBytes)
	if err != nil {
		log.Printf("error creating random filename bytes %v\n", err)
	}
	randFilename := base64.RawURLEncoding.EncodeToString(nameBytes)

	filename := fmt.Sprintf("%s.%s", randFilename, fileExtension)
	newFilepath := filepath.Join(cfg.assetsRoot, filename)
	fmt.Printf("new filepath %v\n", newFilepath)
	newFile, err := os.Create(newFilepath)
	if err != nil {
		log.Printf("error creating thumbnail file %v\n", err)
	}
	number, err := io.Copy(newFile, thumbnailData)
	if err != nil || number == 0 {
		log.Printf("error storing data in newfile %v\n", err)
	}
	newURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)
	//http://localhost:<port>/assets/<videoID>.<file_extension>

	video.ThumbnailURL = &newURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		log.Printf("error updating video in database %v\n", err)
	}

	respondWithJSON(w, http.StatusOK, video)
}
