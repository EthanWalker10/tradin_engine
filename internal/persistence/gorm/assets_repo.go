package gorm

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	k_repo "github.com/duolacloud/crud-core-gorm/repositories"
	"github.com/duolacloud/crud-core/cache"
	"github.com/duolacloud/crud-core/datasource"
	b_mappers "github.com/duolacloud/crud-core/mappers"
	"github.com/duolacloud/crud-core/repositories"
	"github.com/yzimhao/trading_engine/v2/internal/models"
	"github.com/yzimhao/trading_engine/v2/internal/models/types"
	"github.com/yzimhao/trading_engine/v2/internal/persistence"
	"github.com/yzimhao/trading_engine/v2/internal/persistence/gorm/entities"
	"gorm.io/gorm"
)

type gormAssetsRepo struct {
	*repositories.MapperRepository[models.Assets, models.CreateAssets, models.UpdateAssets, entities.Assets, entities.Assets, map[string]any]
	datasource       datasource.DataSource[gorm.DB]
	assetsLogRepo    *gormAssetsLogRepo
	assetsFreezeRepo *gormAssetsFreezeRepo
}

type gormAssetsLogRepo struct {
	*repositories.MapperRepository[models.AssetsLog, models.CreateAssetsLog, models.UpdateAssetsLog, entities.AssetsLog, entities.AssetsLog, map[string]any]
}

type gormAssetsFreezeRepo struct {
	*repositories.MapperRepository[models.AssetsFreeze, models.CreateAssetsFreeze, models.UpdateAssetsFreeze, entities.AssetsFreeze, entities.AssetsFreeze, map[string]any]
}

func NewAssetsRepo(datasource datasource.DataSource[gorm.DB], cache cache.Cache) persistence.AssetsRepository {
	cacheWrapperRepo := repositories.NewCacheRepository(
		k_repo.NewGormCrudRepository[entities.Assets, entities.Assets, map[string]any](datasource),
		cache,
	)

	mapperRepo := repositories.NewMapperRepository(
		cacheWrapperRepo,
		b_mappers.NewJSONMapper[models.Assets, models.CreateAssets, models.UpdateAssets, entities.Assets, entities.Assets, map[string]any](),
	)

	return &gormAssetsRepo{
		MapperRepository: mapperRepo,
		datasource:       datasource,
		assetsLogRepo:    newAssetsLogRepo(datasource, cache),
		assetsFreezeRepo: newAssetsFreezeRepo(datasource, cache),
	}

}

func newAssetsLogRepo(datasource datasource.DataSource[gorm.DB], cache cache.Cache) *gormAssetsLogRepo {
	cacheWrapperRepo := repositories.NewCacheRepository(
		k_repo.NewGormCrudRepository[entities.AssetsLog, entities.AssetsLog, map[string]any](datasource),
		cache,
	)

	mapperRepo := repositories.NewMapperRepository(
		cacheWrapperRepo,
		b_mappers.NewJSONMapper[models.AssetsLog, models.CreateAssetsLog, models.UpdateAssetsLog, entities.AssetsLog, entities.AssetsLog, map[string]any](),
	)

	return &gormAssetsLogRepo{
		MapperRepository: mapperRepo,
	}
}

func newAssetsFreezeRepo(datasource datasource.DataSource[gorm.DB], cache cache.Cache) *gormAssetsFreezeRepo {
	cacheWrapperRepo := repositories.NewCacheRepository(
		k_repo.NewGormCrudRepository[entities.AssetsFreeze, entities.AssetsFreeze, map[string]any](datasource),
		cache,
	)

	mapperRepo := repositories.NewMapperRepository(
		cacheWrapperRepo,
		b_mappers.NewJSONMapper[models.AssetsFreeze, models.CreateAssetsFreeze, models.UpdateAssetsFreeze, entities.AssetsFreeze, entities.AssetsFreeze, map[string]any](),
	)

	return &gormAssetsFreezeRepo{
		MapperRepository: mapperRepo,
	}
}

func (r *gormAssetsRepo) Despoit(ctx context.Context, userId, symbol string, amount string) error {
	return r.transfer(ctx, symbol, entities.SYSTEM_USER_ID, userId, types.Amount(amount), "despoit")

}

func (r *gormAssetsRepo) Withdraw(ctx context.Context, userId, symbol, amount string) error {
	return r.transfer(ctx, symbol, userId, entities.SYSTEM_USER_ID, types.Amount(amount), "withdraw")
}

func (r *gormAssetsRepo) Transfer(ctx context.Context, from, to, symbol, amount string) error {
	return r.transfer(ctx, symbol, from, to, types.Amount(amount), uuid.New().String())
}

func (r *gormAssetsRepo) transfer(ctx context.Context, symbol, from, to string, amount types.Amount, transId string) error {

	db, err := r.datasource.GetDB(ctx)
	if err != nil {
		return errors.Wrap(err, "get gorm db")
	}

	//临时有要求使用原生sql 不使用orm
	rawDb, err := db.DB()
	if err != nil {
		return errors.Wrap(err, "get rawdb")
	}

	fromUser, err := r.queryOne(ctx, rawDb, from, symbol)
	if err != nil {
		return errors.Wrap(err, "query from user")
	}

	toUser, err := r.queryOne(ctx, rawDb, to, symbol)
	if err != nil {
		return errors.Wrap(err, "query to user")
	}

	tx, err := rawDb.Begin()
	if err != nil {
		return errors.Wrap(err, "begin tx")
	}

	fromUser.TotalBalance = fromUser.TotalBalance.Sub(amount)
	fromUser.AvailBalance = fromUser.AvailBalance.Sub(amount)

	toUser.TotalBalance = toUser.TotalBalance.Add(amount)
	toUser.AvailBalance = toUser.AvailBalance.Add(amount)

	if err := r.update(ctx, tx, fromUser); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "update from user")
	}
	if err := r.update(ctx, tx, toUser); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "update to user")
	}

	//create logs
	err = r.createLog(ctx, tx, &entities.AssetsLog{
		UserId:        from,
		Symbol:        symbol,
		BeforeBalance: fromUser.TotalBalance.Add(amount),
		Amount:        amount,
		AfterBalance:  fromUser.TotalBalance,
		TransID:       transId,
		ChangeType:    "despoit",
	})
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "create from log")
	}

	err = r.createLog(ctx, tx, &entities.AssetsLog{
		UserId:        to,
		Symbol:        symbol,
		BeforeBalance: toUser.TotalBalance.Sub(amount),
		Amount:        amount,
		AfterBalance:  toUser.TotalBalance,
		TransID:       transId,
		ChangeType:    "withdraw",
	})

	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "create to log")
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "commit tx")
	}

	return nil
}

func (r *gormAssetsRepo) createLog(ctx context.Context, db *sql.Tx, log *entities.AssetsLog) error {
	stmt, err := db.Prepare("insert into assets_logs (id, user_id, symbol, before_balance, amount, after_balance, trans_id, change_type, info, created_at, updated_at) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)")
	if err != nil {
		return errors.Wrap(err, "prepare insert user")
	}

	_, err = stmt.Exec(uuid.New().String(), log.UserId, log.Symbol, log.BeforeBalance, log.Amount, log.AfterBalance, log.TransID, log.ChangeType, log.Info, time.Now(), time.Now())
	if err != nil {
		return errors.Wrap(err, "exec insert user")
	}

	return nil
}

func (r *gormAssetsRepo) FindOne(ctx context.Context, userId, symbol string) (*entities.Assets, error) {
	db, err := r.datasource.GetDB(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get gorm db")
	}

	rawDb, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "get raw db")
	}

	return r.queryOne(ctx, rawDb, userId, symbol)
}

func (r *gormAssetsRepo) FindAssetHistory(ctx context.Context) ([]entities.AssetsLog, error) {
	db, err := r.datasource.GetDB(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get gorm db")
	}

	rawDb, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "get raw db")
	}

	rows, err := rawDb.QueryContext(ctx, "SELECT * FROM assets_logs limit 10")
	if err != nil {
		return nil, errors.Wrap(err, "query assets logs")
	}

	defer rows.Close()

	var logs []entities.AssetsLog

	for rows.Next() {
		var log entities.AssetsLog
		err := rows.Scan(&log.Id, &log.UserId, &log.Symbol, &log.BeforeBalance, &log.Amount, &log.AfterBalance, &log.TransID, &log.ChangeType, &log.Info, &log.CreatedAt, &log.UpdatedAt)
		if err != nil {
			return nil, errors.Wrap(err, "scan assets log")
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (r *gormAssetsRepo) queryOne(ctx context.Context, rawDb *sql.DB, userId, symbol string) (*entities.Assets, error) {

	// 查询是否存在指定的资产记录
	row := rawDb.QueryRowContext(ctx, "SELECT * FROM assets WHERE user_id = $1 AND symbol = $2 LIMIT 1", userId, symbol)
	var user entities.Assets
	err := row.Scan(&user.Id, &user.UserId, &user.Symbol, &user.TotalBalance, &user.AvailBalance, &user.FreezeBalance, &user.CreatedAt, &user.UpdatedAt)

	// 如果出现数据库查询错误
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "query user")
	}

	if err == sql.ErrNoRows {
		return &entities.Assets{
			UserId: userId,
			Symbol: symbol,
		}, nil
	}

	return &user, nil
}

func (r *gormAssetsRepo) update(ctx context.Context, tx *sql.Tx, user *entities.Assets) error {
	// 查询是否存在指定的资产记录
	row := tx.QueryRowContext(ctx, "SELECT id FROM assets WHERE user_id = $1 AND symbol = $2 LIMIT 1", user.UserId, user.Symbol)
	var id string
	err := row.Scan(&id)

	// 如果出现数据库查询错误
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "query user")
	}

	if err == sql.ErrNoRows {
		// 如果记录不存在，执行插入操作
		_, err := tx.ExecContext(ctx, "INSERT INTO assets (id, user_id, symbol, total_balance, avail_balance, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
			uuid.New().String(), user.UserId, user.Symbol, user.TotalBalance, user.AvailBalance, time.Now(), time.Now())
		if err != nil {
			return errors.Wrap(err, "exec insert user")
		}
	} else {
		// 如果记录存在，执行更新操作
		_, err := tx.ExecContext(ctx, "UPDATE assets SET total_balance = $1, avail_balance = $2, updated_at = $3 WHERE id = $4",
			user.TotalBalance, user.AvailBalance, time.Now(), id)
		if err != nil {
			return errors.Wrap(err, "exec update user")
		}
	}

	return nil
}
