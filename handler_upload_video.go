package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	//"fmt"
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxBytes = 1 << 30
	var fileCloser io.ReadCloser
	fileCloser = http.MaxBytesReader(w, fileCloser, maxBytes)

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

	video, err := cfg.db.GetVideo(videoID)
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorised", err)
		return
	}
	if err != nil {
		log.Printf("error from retrieval of video from database %v\n", err)

	}

	//const maxMemory = 1 << 30
	//err = r.ParseMultipartForm(maxMemory)
	//if err != nil {
	//log.Printf("Error from upload thumbnail handler when parsing %v\n", err)
	//}
	videoFile, header, err := r.FormFile("video")
	if err != nil {
		log.Printf("Error from video handler when extracting video and header %v", err)
	}
	defer videoFile.Close()

	RmediaType := header.Header.Get("Content-Type")
	//ParseMediaType(v string) (mediatype string, params map[string]string, err error)
	mediatype, _, err := mime.ParseMediaType(RmediaType)
	if err != nil {
		log.Printf("error parsing media type %v\n", err)
	}
	if mediatype != "video/mp4" {
		log.Printf("wrong media type \n")
		respondWithError(w, http.StatusBadRequest, "Wrong file type", fmt.Errorf("Wrong file type in Content-Type"))
		return
	}
	// put mulipart.File into a temporary file

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		log.Printf("error creating temporary video file %v\n", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	number, err := io.Copy(tempFile, videoFile)
	if err != nil || number == 0 {
		log.Printf("error storing data in newfile %v\n", err)
	}
	tempFile.Seek(0, io.SeekStart)

	// make a random key for s3 object randstring.ext
	keyBytes := make([]byte, 32)
	_, err = rand.Read(keyBytes)
	if err != nil {
		log.Printf("error creating random filename bytes %v\n", err)
	}
	randKey := base64.RawURLEncoding.EncodeToString(keyBytes)
	keyName := fmt.Sprintf("%s.mp4", randKey)

	// content type mime
	mimeType := "video/mp4"

	s3params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &keyName,
		Body:        tempFile,
		ContentType: &mimeType,
	}
	_, err = cfg.s3Client.PutObject(context.Background(), &s3params)
	//func (c *Client) PutObject(ctx context.Context, params *PutObjectInput, optFns ...func(*Options)) (*PutObjectOutput, error)
	VUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, keyName)
	video.VideoURL = &VUrl
	//Update the VideoURL of the video record in the database with the S3 bucket and key. S3 URLs are in the format https://<bucket-name>.s3.<region>.amazonaws.com/<key>. Make sure you use the correct region and bucket name!
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		log.Printf("error updating video record %v\n", err)
	}
}
