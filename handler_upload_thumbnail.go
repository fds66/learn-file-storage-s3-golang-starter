package main

import (
	"encoding/base64"
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
	//fmt.Printf("media type %s\n", RmediaType)

	imageData, err := io.ReadAll(thumbnailData)
	//fmt.Printf("user id %v\n", userID)
	video, err := cfg.db.GetVideo(videoID)
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorised", err)
	}
	//fmt.Printf("return video struct %+v\n", video)
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
			//data:<media-type>;base64,<data>
				A few examples:

		data:,Hello%2C%20World%21
		The text/plain data Hello, World!. Note how the comma is percent-encoded as %2C, and the space character as %20.

		data:text/plain;base64,SGVsbG8sIFdvcmxkIQ==
		base64-encoded version of the above

		data:text/html,%3Ch1%3EHello%2C%20World%21%3C%2Fh1%3E
		An HTML document with <h1>Hello, World!</h1>

		data:text/html,%3Cscript%3Ealert%28%27hi%27%29%3B%3C%2Fscript%3E
		An HTML document with <script>alert('hi');</script> that executes a JavaScript alert. Note that the closing script tag is required.
	*/

	/* REPLACE all of the videoThumbnails map with encoding the image in the thumbnail url field of the video database
	newThumbnail := thumbnail{
		data:      imageData,
		mediaType: RmediaType,
	}
	*/
	//videoThumbnails[video.ID] = newThumbnail
	//fmt.Printf("new thumbnail struct and entry in map %+v\n", videoThumbnails[video.ID])
	//newURL := fmt.Sprintf("http://localhost:8091/api/thumbnails/%s", video.ID.String())

	encodedThumbnail := base64.StdEncoding.EncodeToString(imageData)

	dataURL := fmt.Sprintf("data:%s;base64,%s", RmediaType, encodedThumbnail)
	video.ThumbnailURL = &dataURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		log.Printf("error updating video in database %v\n", err)
	}

	respondWithJSON(w, http.StatusOK, video)
}
