package models

import "github.com/yzimhao/trading_engine/v2/internal/persistence/gorm/entities"

type AssetsLog struct {
	Base
	UserId     string                    `json:"user_id"`
	Symbol     string                    `json:"symbol"`
	Before     string                    `json:"before"` // 变动前
	Amount     string                    `json:"amount"` // 变动数
	After      string                    `json:"after"`  // 变动后
	TransID    string                    `json:"trans_id"`
	ChangeType entities.AssetsChangeType `json:"change_type"`
	Info       string                    `json:"info"`
}

type CreateAssetsLog struct {
	UserId     string                    `json:"user_id"`
	Symbol     string                    `json:"symbol"`
	Before     string                    `json:"before"` // 变动前
	Amount     string                    `json:"amount"` // 变动数
	After      string                    `json:"after"`  // 变动后
	TransID    string                    `json:"trans_id"`
	ChangeType entities.AssetsChangeType `json:"change_type"`
	Info       string                    `json:"info"`
}

type UpdateAssetsLog struct {
	ID      int64   `json:"id"`
	UserId  *string `json:"user_id"`
	Symbol  *string `json:"symbol"`
	Before  *string `json:"before"` // 变动前
	Amount  *string `json:"amount"` // 变动数
	After   *string `json:"after"`  // 变动后
	TransID *string `json:"trans_id"`
}