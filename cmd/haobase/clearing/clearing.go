package clearing

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"github.com/yzimhao/trading_engine/cmd/haobase/base/symbols"
	"github.com/yzimhao/trading_engine/trading_core"
	"github.com/yzimhao/trading_engine/types"
	"github.com/yzimhao/trading_engine/utils"
	"github.com/yzimhao/trading_engine/utils/app"
)

func Run() {
	//load symbols
	db := app.Database().NewSession()
	defer db.Close()

	var rows []symbols.TradingVarieties
	db.Table(new(symbols.TradingVarieties)).Find(&rows)

	for _, row := range rows {
		run_clearing(row.Symbol)
	}
}

func run_clearing(symbol string) {
	go watch_redis_list(symbol)
}

func watch_redis_list(symbol string) {
	key := types.FormatTradeResult.Format(symbol)
	logrus.Infof("正在监听%s成交日志 结算...", symbol)
	for {
		func() {
			rdc := app.RedisPool().Get()
			defer rdc.Close()

			if n, _ := redis.Int64(rdc.Do("LLen", key)); n == 0 {
				time.Sleep(time.Duration(50) * time.Millisecond)
				return
			}

			raw, _ := redis.Bytes(rdc.Do("Lpop", key))
			logrus.Infof("%s成交记录: %s", symbol, raw)

			go clearing_trade_order(symbol, raw)
		}()

	}
}

func clearing_trade_order(symbol string, raw []byte) {
	var data trading_core.TradeResult
	err := json.Unmarshal(raw, &data)
	if err != nil {
		logrus.Errorf("%s成交日志格式错误: %s %s", symbol, err.Error(), raw)
		return
	}

	lock(data.AskOrderId)
	lock(data.BidOrderId)

	if data.Last == "" {
		go newClean(data)
	} else {
		go func() {
			for {
				time.Sleep(time.Duration(50) * time.Millisecond)
				logrus.Infof("等待订单 %s 其他订单结算完成....", data.Last)
				if getlock(data.Last) == 1 {
					newClean(data)
					break
				}
			}
		}()
	}

	//通知kline系统
	rdc := app.RedisPool().Get()
	defer rdc.Close()
	quote_key := types.FormatQuoteTradeResult.Format(symbol)
	if _, err := rdc.Do("RPUSH", quote_key, raw); err != nil {
		logrus.Errorf("rpush %s err: %s", quote_key, err.Error())
	}
}

func generate_trading_id(ask, bid string) string {
	times := time.Now().Format("060102")
	hash := utils.Hash256(fmt.Sprintf("%s%s", ask, bid))
	return fmt.Sprintf("T%s%s", times, hash[0:17])
}

func lock(order_id string) {
	rdc := app.RedisPool().Get()
	defer rdc.Close()

	key := fmt.Sprintf("clearing.lock.%s", order_id)
	rdc.Do("INCR", key)
}

func unlock(order_id string) {
	rdc := app.RedisPool().Get()
	defer rdc.Close()

	key := fmt.Sprintf("clearing.lock.%s", order_id)
	if _, err := rdc.Do("DECR", key); err != nil {
		logrus.Warnf("clearing unlock %s err: %s", order_id, err.Error())
	}
	if _, err := rdc.Do("Expire", key, 300); err != nil {
		logrus.Warnf("clearing unlock %s set expire err: %s", order_id, err.Error())
	}
}

func getlock(order_id string) int64 {
	rdc := app.RedisPool().Get()
	defer rdc.Close()

	key := fmt.Sprintf("clearing.lock.%s", order_id)
	n, _ := redis.Int64(rdc.Do("GET", key))
	return n
}
