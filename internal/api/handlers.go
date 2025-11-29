package api

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"project_sem/internal/archive"
	"project_sem/internal/csv"
	"project_sem/internal/db"
	"project_sem/internal/models"
	"strconv"
	"time"
)

type PricesResponse struct {
	TotalCount      int     `json:"total_count"`
	DuplicatesCount int     `json:"duplicates_count"`
	TotalItems      int     `json:"total_items"`
	TotalCategories int     `json:"total_categories"`
	TotalPrice      float64 `json:"total_price"`
}

func handlePostPrices(pg *db.PG, w http.ResponseWriter, r *http.Request) {
	archiveKind := r.URL.Query().Get("type")
	if archiveKind == "" {
		archiveKind = "zip"
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "failed to read file", http.StatusBadRequest)
		log.Printf("failed to read file: %v", err)
		return
	}
	defer file.Close()

	var reader archive.ArchiveReader

	switch archiveKind {
	case "zip":
		reader, err = archive.NewZipReader(file)
	case "tar":
		reader = archive.NewTarReader(file)
	default:
		http.Error(w, "unknown archive type", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rc, err := reader.ReadDataCSV()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer rc.Close()

	var (
		rowCount int
		insCount int
		errCount int
		dupCount int
	)

	parseErr := csv.ParseCSV(rc, func(line int, price models.Price, err error) {
		rowCount++
		if err != nil {
			errCount++
			log.Printf("line %d parse error: %v\n", line, err)
			return
		}

		inserted, err := db.InsertPrice(pg, price)
		if err != nil {
			errCount++
			log.Printf("line %d insert error: %v\n", line, err)
			return
		}
		if !inserted {
			dupCount++
			log.Printf("line %d duplicate: %+v\n", line, price)
		}
		insCount++
	})

	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Processed %d rows (%d inserted, %d errors, %d duplicates)\n", rowCount, insCount, errCount, dupCount)
	totalItems, err := db.SelectTotalItems(pg)
	if err != nil {
		log.Printf("failed to fetch total items: %v", err)
		http.Error(w, "failed to fetch total items", http.StatusInternalServerError)
		return
	}

	totalCategories, err := db.SelectTotalCategories(pg)
	if err != nil {
		log.Printf("failed to fetch total categories: %v", err)
		http.Error(w, "failed to fetch total categories", http.StatusInternalServerError)
		return
	}

	totalPrice, err := db.SelectTotalPrice(pg)
	if err != nil {
		log.Printf("failed to fetch total price: %v", err)
		http.Error(w, "failed to fetch total price", http.StatusInternalServerError)
		return
	}

	resp := PricesResponse{
		TotalCount:      rowCount,
		DuplicatesCount: dupCount,
		TotalItems:      totalItems,
		TotalCategories: totalCategories,
		TotalPrice:      totalPrice,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func PostPrices(pg *db.PG) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handlePostPrices(pg, w, r)
	}
}

func handleGetPrices(pg *db.PG, w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	minStr := r.URL.Query().Get("min")
	maxStr := r.URL.Query().Get("max")

	if startStr == "" || endStr == "" || minStr == "" || maxStr == "" {
		http.Error(w, "missing required query parameters", http.StatusBadRequest)
		return
	}

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		http.Error(w, "invalid start date", http.StatusBadRequest)
		return
	}

	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		http.Error(w, "invalid end date", http.StatusBadRequest)
		return
	}

	min, err := strconv.Atoi(minStr)
	if err != nil || min < 0 {
		http.Error(w, "invalid min value", http.StatusBadRequest)
		return
	}
	max, err := strconv.Atoi(maxStr)
	if err != nil || max < 0 {
		http.Error(w, "invalid max value", http.StatusBadRequest)
		return
	}

	rows, err := db.SelectByFilter(start, end, min, max, pg)
	if err != nil {
		http.Error(w, "failed to fetch data", http.StatusInternalServerError)
		return
	}

	var csvBuf bytes.Buffer
	if err := csv.WriteCSV(&csvBuf, rows); err != nil {
		http.Error(w, "csv generation failed", http.StatusInternalServerError)
		return
	}

	zipData, err := archive.WriteDataZip("data.csv", csvBuf.Bytes())
	if err != nil {
		http.Error(w, "zip generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=prices.zip")
	w.Write(zipData)
}

func GetPrices(pg *db.PG) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleGetPrices(pg, w, r)
	}
}
