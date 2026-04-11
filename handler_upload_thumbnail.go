package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

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
	/*
			type File interface {
			io.Reader
			io.ReaderAt
			io.Seeker
			io.Closer
		}

			type FileHeader struct {
			Filename string
			Header   textproto.MIMEHeader
			Size     int64
			// contains filtered or unexported fields


		}
	*/
	RmediaType := header.Header.Get("Content-Type")
	fmt.Printf("media type %s\n", RmediaType)

	imageData, err := io.ReadAll(thumbnailData)
	fmt.Printf("user id %v\n", userID)
	video, err := cfg.db.GetVideo(videoID)
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorised", err)
	}
	fmt.Printf("return video struct %+v\n", video)
	if err != nil {
		log.Printf("error from retrieval of video from database %v\n", err)

	}
	/*
			type Video struct {
			ID           uuid.UUID `json:"id"`
			CreatedAt    time.Time `json:"created_at"`
			UpdatedAt    time.Time `json:"updated_at"`
			ThumbnailURL *string   `json:"thumbnail_url"`
			VideoURL     *string   `json:"video_url"`
			CreateVideoParams
		}

		type CreateVideoParams struct {
			Title       string    `json:"title"`
			Description string    `json:"description"`
			UserID      uuid.UUID `json:"user_id"`
		}
			type thumbnail struct {
			data      []byte
			mediaType string
		}
	*/
	newThumbnail := thumbnail{
		data:      imageData,
		mediaType: RmediaType,
	}
	videoThumbnails[video.ID] = newThumbnail
	//fmt.Printf("new thumbnail struct and entry in map %+v\n", videoThumbnails[video.ID])
	newURL := fmt.Sprintf("http://localhost:<port>/api/thumbnails/{videoID}%s", video.ID.String())

	video.ThumbnailURL = &newURL
	fmt.Printf("new url %s and updated video struct %+v\n", newURL, video)
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		log.Printf("error updating video in database %v\n", err)
	}

	respondWithJSON(w, http.StatusOK, video)
}
