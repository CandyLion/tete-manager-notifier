package models

import "time"

type Position struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	CarID     int16     `gorm:"column:car_id"`
	DriveID   *uint     `gorm:"column:drive_id"`
	Date      time.Time `gorm:"column:date"`
	Latitude  float64   `gorm:"column:latitude"`
	Longitude float64   `gorm:"column:longitude"`
}

func (Position) TableName() string { return "positions" }
