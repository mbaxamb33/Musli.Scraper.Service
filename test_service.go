// // test_service.go - Simple test script for the scraper service
package main

// import (
// 	"bytes"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net/http"
// 	"time"

// 	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
// )

// const (
// 	BaseURL = "http://localhost:8081"
// )

// func main() {
// 	fmt.Println("🧪 Testing Musli Scraper Service")
// 	fmt.Println("==================================")

// 	// Wait for service to be ready
// 	if !waitForService() {
// 		log.Fatal("❌ Service is not ready")
// 	}

// 	// Run tests
// 	tests := []TestCase{
// 		{"Health Check", testHealthCheck},
// 		{"Worker Status", testWorkerStatus},
// 		{"Job Metrics", testJobMetrics},
// 		{"Simple Scraping Job", testSimpleScrapingJob},
// 		{"Scraping Job with Options", testScrapingJobWithOptions},
// 		{"List Jobs", testListJobs},
// 		{"Queue Status", testQueueStatus},
// 	}

// 	passed := 0
// 	for _, test := range tests {
// 		fmt.Printf("\n🔍 Testing: %s\n", test.Name)
// 		if test.Func() {
// 			fmt.Printf("✅ %s - PASSED\n", test.Name)
// 			passed++
// 		} else {
// 			fmt.Printf("❌ %s - FAILED\n", test.Name)
// 		}
// 	}

// 	fmt.Printf("\n📊 Results: %d/%d tests passed\n", passed, len(tests))

// 	if passed == len(tests) {
// 		fmt.Println("🎉 All tests passed!")
// 	} else {
// 		fmt.Println("⚠️  Some tests failed. Check the logs above.")
// 	}
// }

// type TestCase struct {
// 	Name string
// 	Func func() bool
// }

// func waitForService() bool {
// 	fmt.Print("⏳ Waiting for service to be ready")
// 	for i := 0; i < 30; i++ {
// 		resp, err := http.Get(BaseURL + "/health")
// 		if err == nil && resp.StatusCode == 200 {
// 			resp.Body.Close()
// 			fmt.Println(" ✅")
// 			return true
// 		}
// 		if resp != nil {
// 			resp.Body.Close()
// 		}
// 		fmt.Print(".")
// 		time.Sleep(1 * time.Second)
// 	}
// 	fmt.Println(" ❌")
// 	return false
// }

// func testHealthCheck() bool {
// 	resp, err := http.Get(BaseURL + "/health")
// 	if err != nil {
// 		fmt.Printf("   Error: %v\n", err)
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		fmt.Printf("   Expected status 200, got %d\n", resp.StatusCode)
// 		return false
// 	}

// 	var health map[string]interface{}
// 	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
// 		fmt.Printf("   Failed to decode response: %v\n", err)
// 		return false
// 	}

// 	fmt.Printf("   Service: %v, Status: %v\n", health["service"], health["status"])
// 	return health["status"] == "ok"
// }

// func testWorkerStatus() bool {
// 	resp, err := http.Get(BaseURL + "/api/workers/status")
// 	if err != nil {
// 		fmt.Printf("   Error: %v\n", err)
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		fmt.Printf("   Expected status 200, got %d\n", resp.StatusCode)
// 		return false
// 	}

// 	var status map[string]interface{}
// 	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
// 		fmt.Printf("   Failed to decode response: %v\n", err)
// 		return false
// 	}

// 	fmt.Printf("   Workers: %v/%v active\n", status["active_workers"], status["worker_count"])
// 	return status["is_running"] == true
// }

// func testJobMetrics() bool {
// 	resp, err := http.Get(BaseURL + "/api/metrics")
// 	if err != nil {
// 		fmt.Printf("   Error: %v\n", err)
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		fmt.Printf("   Expected status 200, got %d\n", resp.StatusCode)
// 		return false
// 	}

// 	var metrics map[string]interface{}
// 	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
// 		fmt.Printf("   Failed to decode response: %v\n", err)
// 		return false
// 	}

// 	fmt.Printf("   Total jobs: %v\n", metrics["total_jobs"])
// 	return true
// }

// func testSimpleScrapingJob() bool {
// 	job := models.ScrapingJobRequest{
// 		URL: "https://druidai.com",
// 		Options: models.ScrapingOptions{
// 			WaitForJS: false,
// 			Timeout:   30 * time.Second,
// 		},
// 	}

// 	jobID, success := createJob(job)
// 	if !success {
// 		return false
// 	}

// 	fmt.Printf("   Created job: %s\n", jobID)

// 	// Wait for job to complete
// 	return waitForJobCompletion(jobID, 60*time.Second)
// }

// func testScrapingJobWithOptions() bool {
// 	job := models.ScrapingJobRequest{
// 		URL:      "https://druidai.com",
// 		Priority: 8,
// 		Options: models.ScrapingOptions{
// 			WaitForJS:       true,
// 			Timeout:         45 * time.Second,
// 			ScrollToBottom:  true,
// 			WaitForSelector: "body",
// 		},
// 		CallbackURL: "https://httpbin.org/post",
// 	}

// 	jobID, success := createJob(job)
// 	if !success {
// 		return false
// 	}

// 	fmt.Printf("   Created high-priority job: %s\n", jobID)

// 	// Just check if job was created successfully, don't wait for completion
// 	// since some test URLs might not be accessible
// 	return checkJobExists(jobID)
// }

// func testListJobs() bool {
// 	resp, err := http.Get(BaseURL + "/api/jobs?limit=5")
// 	if err != nil {
// 		fmt.Printf("   Error: %v\n", err)
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		fmt.Printf("   Expected status 200, got %d\n", resp.StatusCode)
// 		return false
// 	}

// 	var result map[string]interface{}
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		fmt.Printf("   Failed to decode response: %v\n", err)
// 		return false
// 	}

// 	jobs, ok := result["jobs"].([]interface{})
// 	if !ok {
// 		fmt.Printf("   No jobs array in response\n")
// 		return false
// 	}

// 	fmt.Printf("   Found %d jobs\n", len(jobs))
// 	return true
// }

// func testQueueStatus() bool {
// 	resp, err := http.Get(BaseURL + "/api/queue/status")
// 	if err != nil {
// 		fmt.Printf("   Error: %v\n", err)
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		fmt.Printf("   Expected status 200, got %d\n", resp.StatusCode)
// 		return false
// 	}

// 	var status map[string]interface{}
// 	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
// 		fmt.Printf("   Failed to decode response: %v\n", err)
// 		return false
// 	}

// 	fmt.Printf("   Queue available: %v\n", status["available"])
// 	return true
// }

// // Helper functions

// func createJob(job models.ScrapingJobRequest) (string, bool) {
// 	jsonData, err := json.Marshal(job)
// 	if err != nil {
// 		fmt.Printf("   Failed to marshal job: %v\n", err)
// 		return "", false
// 	}

// 	resp, err := http.Post(BaseURL+"/api/jobs", "application/json", bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		fmt.Printf("   Failed to create job: %v\n", err)
// 		return "", false
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 201 {
// 		body, _ := io.ReadAll(resp.Body)
// 		fmt.Printf("   Expected status 201, got %d: %s\n", resp.StatusCode, string(body))
// 		return "", false
// 	}

// 	var result map[string]interface{}
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		fmt.Printf("   Failed to decode job response: %v\n", err)
// 		return "", false
// 	}

// 	jobID, ok := result["id"].(string)
// 	if !ok {
// 		fmt.Printf("   No job ID in response\n")
// 		return "", false
// 	}

// 	return jobID, true
// }

// func waitForJobCompletion(jobID string, timeout time.Duration) bool {
// 	deadline := time.Now().Add(timeout)

// 	for time.Now().Before(deadline) {
// 		resp, err := http.Get(BaseURL + "/api/jobs/" + jobID)
// 		if err != nil {
// 			fmt.Printf("   Error checking job status: %v\n", err)
// 			return false
// 		}

// 		var job map[string]interface{}
// 		json.NewDecoder(resp.Body).Decode(&job)
// 		resp.Body.Close()

// 		status, ok := job["status"].(string)
// 		if !ok {
// 			fmt.Printf("   No status in job response\n")
// 			return false
// 		}

// 		fmt.Printf("   Job status: %s\n", status)

// 		switch status {
// 		case "completed":
// 			if results, ok := job["results"]; ok && results != nil {
// 				fmt.Printf("   Job completed with results\n")
// 				return true
// 			}
// 			fmt.Printf("   Job completed but no results\n")
// 			return false
// 		case "failed":
// 			if errorMsg, ok := job["error"].(string); ok {
// 				fmt.Printf("   Job failed: %s\n", errorMsg)
// 			}
// 			return false
// 		case "canceled":
// 			fmt.Printf("   Job was canceled\n")
// 			return false
// 		}

// 		time.Sleep(2 * time.Second)
// 	}

// 	fmt.Printf("   Job did not complete within timeout\n")
// 	return false
// }

// func checkJobExists(jobID string) bool {
// 	resp, err := http.Get(BaseURL + "/api/jobs/" + jobID)
// 	if err != nil {
// 		fmt.Printf("   Error checking job: %v\n", err)
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	return resp.StatusCode == 200
// }
