package db

import (
	"fmt"
	"log"
	"time"

	"github.com/wen-ryon/tete-manager-notifier/internal/config"
	"github.com/wen-ryon/tete-manager-notifier/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(cfg *config.Config) error {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		cfg.DBHost, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBPort)

	newLogger := logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             1 * time.Second,
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return nil
}

// GetCarName 获取车辆名称
func GetCarName(carID int) (string, error) {
	var car models.Car
	err := DB.Where("id = ?", carID).First(&car).Error
	if err != nil {
		return "我的 Tesla", err
	}
	return car.Name, nil
}

type DriveWithSOC struct {
	models.Drive
	StartSOC float64 `gorm:"column:start_soc"`
	EndSOC   float64 `gorm:"column:end_soc"`
}

type DriveDetail struct {
	DriveWithSOC
	StartLatitude  float64 `gorm:"column:start_latitude"`
	StartLongitude float64 `gorm:"column:start_longitude"`
	EndLatitude    float64 `gorm:"column:end_latitude"`
	EndLongitude   float64 `gorm:"column:end_longitude"`
}

func GetLatestDrive(carID int) (*DriveWithSOC, error) {
	var result DriveWithSOC

	err := DB.Table("drives d").
		Select(`d.*, 
				COALESCE(start_pos.usable_battery_level, start_pos.battery_level, 0) as start_soc,
				COALESCE(end_pos.usable_battery_level, end_pos.battery_level, 0) as end_soc`).
		Joins("LEFT JOIN positions start_pos ON d.start_position_id = start_pos.id").
		Joins("LEFT JOIN positions end_pos ON d.end_position_id = end_pos.id").
		Where("d.car_id = ?", carID).
		Order("d.id desc").
		First(&result).Error

	return &result, err
}

func GetDriveDetail(carID int, driveID uint) (*DriveDetail, error) {
	var result DriveDetail

	err := DB.Table("drives d").
		Select(`d.*,
				COALESCE(start_pos.usable_battery_level, start_pos.battery_level, 0) as start_soc,
				COALESCE(end_pos.usable_battery_level, end_pos.battery_level, 0) as end_soc,
				COALESCE(start_pos.latitude, 0) as start_latitude,
				COALESCE(start_pos.longitude, 0) as start_longitude,
				COALESCE(end_pos.latitude, 0) as end_latitude,
				COALESCE(end_pos.longitude, 0) as end_longitude`).
		Joins("LEFT JOIN positions start_pos ON d.start_position_id = start_pos.id").
		Joins("LEFT JOIN positions end_pos ON d.end_position_id = end_pos.id").
		Where("d.car_id = ? AND d.id = ?", carID, driveID).
		First(&result).Error

	return &result, err
}

func GetDrivePositions(carID int, driveID uint, startDate, endDate time.Time) ([]models.Position, error) {
	var positions []models.Position

	err := DB.
		Where("car_id = ? AND drive_id = ?", carID, driveID).
		Order("date asc").
		Find(&positions).Error
	if err != nil {
		return nil, err
	}

	if len(positions) > 0 {
		return positions, nil
	}

	return GetDrivePositionsByTime(carID, startDate, endDate)
}

func GetDrivePositionsByTime(carID int, startDate, endDate time.Time) ([]models.Position, error) {
	var positions []models.Position

	err := DB.
		Where("car_id = ? AND date BETWEEN ? AND ?", carID, startDate, endDate).
		Order("date asc").
		Find(&positions).Error

	return positions, err
}

// GetLatestCharge 联表查询 charges 表获取快充判断字段
func GetLatestCharge(carID int) (*models.Charge, error) {
	type Result struct {
		models.Charge
		FastChargerPresent *bool  `gorm:"column:fast_charger_present"`
		ChargerPhases      *int16 `gorm:"column:charger_phases"`
		ChargerPower       int16  `gorm:"column:charger_power"`
	}

	var result Result

	err := DB.Table("charging_processes cp").
		Select(`cp.*,
				c.fast_charger_present,
				c.charger_phases,
				c.charger_power`).
		Joins("LEFT JOIN charges c ON c.charging_process_id = cp.id").
		Where("cp.car_id = ?", carID).
		Order("cp.id desc").
		First(&result).Error

	if err != nil {
		return nil, err
	}

	// 把查询到的字段复制到 Charge struct
	charge := &result.Charge
	charge.FastChargerPresent = result.FastChargerPresent
	charge.ChargerPhases = result.ChargerPhases
	charge.ChargerPower = result.ChargerPower

	return charge, nil
}
