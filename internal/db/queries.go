package db

import (
	"fmt"
	"project_sem/internal/models"
	"time"
)

const (
	insertPriceQuery = `INSERT INTO prices (id, name, category, price, create_date)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING;`
	selectTotalItemsQuery      = `SELECT COUNT(1) FROM prices;`
	selectTotalPriceQuery      = `SELECT SUM(price) FROM prices;`
	selectTotalCategoriesQuery = `SELECT COUNT(DISTINCT category) FROM prices;`
	selectByFilterQuery        = `SELECT * FROM prices
WHERE (create_date BETWEEN $1 AND $2)
AND (price BETWEEN $3 AND $4)`
)

func InsertPrice(pg *PG, price models.Price) (bool, error) {
	result, err := pg.DB.Exec(insertPriceQuery, price.ID, price.Name, price.Category, price.Price, price.CreatedAt)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

func SelectTotalItems(pg *PG) (int, error) {
	var count int
	if err := pg.DB.QueryRow(selectTotalItemsQuery).Scan(&count); err != nil {
		return 0, fmt.Errorf("SelectTotalItems: %v", err)
	}
	return count, nil
}

func SelectTotalPrice(pg *PG) (float64, error) {
	var sum float64
	if err := pg.DB.QueryRow(selectTotalPriceQuery).Scan(&sum); err != nil {
		return 0, fmt.Errorf("SelectTotalPrice: %v", err)
	}
	return sum, nil
}

func SelectTotalCategories(pg *PG) (int, error) {
	var count int
	if err := pg.DB.QueryRow(selectTotalCategoriesQuery).Scan(&count); err != nil {
		return 0, fmt.Errorf("SelectTotalCategories: %v", err)
	}
	return count, nil
}

func SelectByFilter(start time.Time, end time.Time, min int, max int, pg *PG) ([]models.Price, error) {
	rows, err := pg.DB.Query(selectByFilterQuery, start, end, min, max)
	if err != nil {
		return nil, fmt.Errorf("SelectByFilter query: %v", err)
	}
	defer rows.Close()

	var prices []models.Price

	for rows.Next() {
		var p models.Price
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Price, &p.CreatedAt); err != nil {
			return prices, fmt.Errorf("SelectByFilter scan: %v", err)
		}
		prices = append(prices, p)
	}
	if err = rows.Err(); err != nil {
		return prices, fmt.Errorf("SelectByFilter rows: %v", err)
	}
	return prices, nil
}
