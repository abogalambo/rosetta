package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"rosetta/models"
)

var client *mongo.Client
var s3Client *s3.S3
var s3Bucket string
var s3Endpoint string
var s3PublicHost string

func main() {
	// Load environment variables
	databaseURL := os.Getenv("DATABASE_URL")
	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket = os.Getenv("S3_BUCKET")
	s3Endpoint = os.Getenv("S3_ENDPOINT")
	s3PublicHost = os.Getenv("S3_PUBLIC_URL")

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	client, err = mongo.Connect(ctx, options.Client().ApplyURI(databaseURL))
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// Initialize AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(awsRegion),
		Credentials:      credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, ""),
		Endpoint:         aws.String(s3Endpoint),
		S3ForcePathStyle: aws.Bool(true), // Required for LocalStack
	})
	if err != nil {
		log.Fatal(err)
	}

	// Initialize S3 client
	s3Client = s3.New(sess)

	// Create bucket if it doesn't exist
	_, err = s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(s3Bucket),
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create a new router
	r := mux.NewRouter()

	// Define routes
	r.HandleFunc("/stories", createStory).Methods("POST")
	r.HandleFunc("/stories/{id}", deleteStory).Methods("DELETE")
	r.HandleFunc("/stories/{id}", updateStory).Methods("PUT")
	r.HandleFunc("/stories/{storyId}/segments/{segmentId}/audio", generateAudioUploadURL).Methods("POST")
	r.HandleFunc("/health", healthCheck).Methods("GET")

	// Start the server
	http.Handle("/", r)
	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createStory(w http.ResponseWriter, r *http.Request) {
	var story models.Story
	err := json.NewDecoder(r.Body).Decode(&story)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Initialize Segments to an empty array if it's nil
	if story.Segments == nil {
		story.Segments = []models.Segment{}
	}

	story.ID = primitive.NewObjectID()
	story.CreatedAt = time.Now()
	collection := client.Database("rosetta").Collection("stories")
	_, err = collection.InsertOne(context.Background(), story)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(story)
}

func deleteStory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid story ID", http.StatusBadRequest)
		return
	}

	collection := client.Database("rosetta").Collection("stories")
	_, err = collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateStory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid story ID", http.StatusBadRequest)
		return
	}

	var story models.Story
	err = json.NewDecoder(r.Body).Decode(&story)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Ensure segments have IDs
	for i, segment := range story.Segments {
		if segment.ID.IsZero() {
			story.Segments[i].ID = primitive.NewObjectID()
		}
	}

	collection := client.Database("rosetta").Collection("stories")
	update := bson.M{
		"$set": bson.M{
			"title":        story.Title,
			"segments":     story.Segments,
			"is_published": story.IsPublished,
		},
	}

	_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var updatedStory models.Story
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&updatedStory)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedStory)
}

func generateAudioUploadURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	storyID := vars["storyId"]
	segmentID := vars["segmentId"]

	objectName := fmt.Sprintf("%s/%s/audio", storyID, segmentID)

	// Generate a pre-signed URL for PUT operation
	req, _ := s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(objectName),
	})
	presignedURL, err := req.Presign(15 * time.Minute)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	presignedURL = strings.Replace(presignedURL, s3Endpoint, s3PublicHost, 1)

	publicURL := fmt.Sprintf("%s/%s/%s", s3PublicHost, s3Bucket, objectName)

	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false) // Disable HTML escaping
	encoder.Encode(map[string]string{
		"upload_url": presignedURL,
		"public_url": publicURL,
	})
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
