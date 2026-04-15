package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"context"

	"log"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
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
	var prefix string
	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	switch aspectRatio {
	case "16:9":
		prefix = "landscape"
	case "9:16":
		prefix = "portrait"
	default:
		prefix = "other"
	}

	// make a random key (filename) for s3 object randstring.ext
	keyBytes := make([]byte, 32)
	_, err = rand.Read(keyBytes)
	if err != nil {
		log.Printf("error creating random filename bytes %v\n", err)
	}
	randKey := base64.RawURLEncoding.EncodeToString(keyBytes)
	keyName := fmt.Sprintf("%s/%s.mp4", prefix, randKey)

	//VUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, keyName)
	videoBucketKey := cfg.s3Bucket + "," + keyName
	video.VideoURL = &videoBucketKey
	// now use a shortlived presigned url for returning video object so will store bucket and key in the database
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		log.Printf("error updating video record %v\n", err)
	}

	// content type mime
	mimeType := "video/mp4"
	fastStartFile, err := processVideoForFastStart(tempFile.Name())
	fastStartFileReader, err := os.Open(fastStartFile)
	if err != nil {
		log.Printf("error opening fast start file %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "", err)
	}
	defer os.Remove(fastStartFileReader.Name())
	defer fastStartFileReader.Close()

	s3params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &keyName,
		Body:        fastStartFileReader,
		ContentType: &mimeType,
	}
	_, err = cfg.s3Client.PutObject(context.Background(), &s3params)
	//func (c *Client) PutObject(ctx context.Context, params *PutObjectInput, optFns ...func(*Options)) (*PutObjectOutput, error)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading file to S3", err)
		return
	}

	signedVideo, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating new video object with presigned url", err)
		return
	}
	respondWithJSON(w, http.StatusOK, signedVideo)

}

func getVideoAspectRatio(filePath string) (string, error) {

	execCommand := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var newByteBuffer bytes.Buffer
	execCommand.Stdout = &newByteBuffer
	execCommand.Run()
	type WidthHeight struct {
		Streams []struct {
			Width  int `json:"width,omitempty"`
			Height int `json:"height,omitempty"`
		}
	}
	var widthHeightData WidthHeight
	if err := json.Unmarshal(newByteBuffer.Bytes(), &widthHeightData); err != nil {
		log.Printf("error creating random filename bytes %v\n", err)
		return "", err
	}
	w := float64(widthHeightData.Streams[0].Width)
	h := float64(widthHeightData.Streams[0].Height)
	ratio := w / h
	tolerance := 0.1
	if math.Abs(ratio-float64(16.0/9.0)) < tolerance {
		return "16:9", nil
	}
	if math.Abs(ratio-float64(9.0/16.0)) < tolerance {
		return "9:16", nil
	} else {
		return "other", nil
	}

}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := fmt.Sprintf("%s.processing", filePath)
	execCommand := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	execCommand.Run()
	return outputFilePath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	params := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	s3PresignClient := s3.NewPresignClient(s3Client)
	presignedRequest, err := s3PresignClient.PresignGetObject(context.Background(), &params, s3.WithPresignExpires(expireTime))
	if err != nil {
		log.Printf("error creating presignedurl %v\n", err)
		return "", err
	}
	return presignedRequest.URL, nil

}

/*
type PresignClient struct {
	// contains filtered or unexported fields
}
	func NewPresignClient(c *Client, optFns ...func(*PresignOptions)) *PresignClient
NewPresignClient generates a presign client using provided API Client and presign options
	func (*PresignClient) PresignGetObject ¶
added in v0.30.0
func (c *PresignClient) PresignGetObject(ctx context.Context, params *GetObjectInput, optFns ...func(*PresignOptions)) (*v4.PresignedHTTPRequest, error)
func WithPresignExpires(dur time.Duration) func(*PresignOptions)


*/

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {

	if video.VideoURL == nil {
		log.Printf("no videoUrl recorded, returning unaltered video \n")

		return video, nil
	}
	expireTime, err := time.ParseDuration("5m")
	if err != nil {
		log.Printf("error creating expire time %v\n", err)
		return database.Video{}, err
	}
	fmt.Printf("url field  s %s\n", *video.VideoURL)
	if len(*video.VideoURL) == 0 {
		log.Printf("error empty string \n")
		err = fmt.Errorf("error empty string")
		return database.Video{}, err
	}
	if !strings.Contains(*video.VideoURL, ",") {
		log.Printf("error no comma in string \n")
		err = fmt.Errorf("error no comma in string")
		return database.Video{}, err
	}
	bucketKey := strings.Split(*video.VideoURL, ",")
	//fmt.Printf("bucket, key, url field  %s %s %s\n", bucketKey[0], bucketKey[1], *video.VideoURL)
	presignedUrl, err := generatePresignedURL(cfg.s3Client, bucketKey[0], bucketKey[1], expireTime)
	if err != nil {
		log.Printf("error creating presignedurl %v\n", err)
		return database.Video{}, err
	}
	//fmt.Printf("presignedUrl %s", presignedUrl)
	video.VideoURL = &presignedUrl
	return video, nil

}
