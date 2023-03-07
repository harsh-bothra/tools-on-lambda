package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/joho/godotenv"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

var db *gorm.DB
var err error
var output chan Job

type Job struct {
	gorm.Model `json:"-"`
	CMDString  string    `json:"cmd_string"`
	Status     int       `json:"status"`
	Worker     string    `json:"worker"`
	JobId      uuid.UUID `gorm:"type:uuid" json:"job_id"`
	Output     string    `json:"output"` // save job_output_file
}

// BeforeCreate will set a UUID in the job_id column
func (job *Job) BeforeCreate(scope *gorm.Scope) error {
	uuid := uuid.NewV4()
	return scope.SetColumn("job_id", uuid)
}

type Response struct {
	Message string
	Error   string
}

func init() {
	if os.Getenv("ENV") != "PROD" {
		envFile := ".env"
		log.Info("loading env file : %s", envFile)
		err := godotenv.Load(envFile)
		if err != nil && !os.IsNotExist(err) {
			log.Error("Error loading .env file")
		}
	}
	log.Info("Setting up new database!!!")
	dbUsername := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	sslMode := os.Getenv("SSL_MODE")

	connectString := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s", dbHost, dbPort, dbUsername, dbName, dbPassword, sslMode)
	log.Info(connectString)
	db, err = gorm.Open("postgres", connectString)
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&Job{})
	log.SetFormatter(&log.JSONFormatter{})
	log.Info("Database Initialized")
}

func StatusUpdater(output chan Job) {
	for {
		select {
		case job := <-output:
			log.Info("Job Completed : ", job.JobId)
			db.Model(&job).Updates(&job)
		}
	}
}

func getJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var jobs []Job
	if result := db.Find(&jobs); result.Error != nil {
		sendErrorResponse(w, "Error retrieving jobs", err)
		return
	}
	json.NewEncoder(w).Encode(jobs)
}

func getJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	job_id := vars["job_id"]
	var job Job

	// basic validation for UUID job_id
	if _, err := uuid.FromString(job_id); err != nil {
		sendErrorResponse(w, "Invalid job id "+job_id, err)
		return
	}

	if result := db.Where("job_id = ?", job_id).First(&job); result.Error != nil {
		sendErrorResponse(w, "Error retrieving job with "+job_id, result.Error)
		return
	}

	json.NewEncoder(w).Encode(job)
}

func createJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var job Job
	_ = json.NewDecoder(r.Body).Decode(&job)

	if result := db.Save(&job); result.Error != nil {
		sendErrorResponse(w, fmt.Sprintf("Error creating job  %+v", job), err)
		return
	}

	// start a worker for this
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Worker(job, output)
	}()

	if err != nil {
		sendErrorResponse(w, fmt.Sprintf("Error creating job  %+v", job), err)
	}

	json.NewEncoder(w).Encode(job.JobId)
	wg.Wait()
}

// LoggingMiddleware - adds middleware around endpoints
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.WithFields(
			log.Fields{
				"Method": r.Method,
				"Path":   r.URL.Path,
			}).Info("Handled request")
		next.ServeHTTP(w, r)
	})
}

func health_check(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"ok": 1})
}

func main() {
	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Not found", r.RequestURI)
		http.Error(w, fmt.Sprintf("Not found: %s", r.RequestURI), http.StatusNotFound)
	})
	r.Use(LoggingMiddleware)
	r.HandleFunc("/health_check", health_check).Methods("GET")
	r.HandleFunc("/job", getJobs).Methods("GET")
	r.HandleFunc("/job/{job_id}", getJob).Methods("GET")
	r.HandleFunc("/job", createJob).Methods("POST")

	output = make(chan Job, 100)
	// Create a status updater function
	go StatusUpdater(output)
	adapter := gorillamux.NewV2(r)
	lambda.Start(adapter.ProxyWithContext)
}

func sendErrorResponse(w http.ResponseWriter, message string, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	if err := json.NewEncoder(w).Encode(Response{Message: message, Error: err.Error()}); err != nil {
		panic(err)
	}
}
