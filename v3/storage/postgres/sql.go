package postgres

import (
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

const schema = `CREATE TABLE IF NOT EXISTS "exchanges"
(
    id                INT PRIMARY KEY,
    name              TEXT UNIQUE NOT NULL,
    trading_fee_maker FLOAT,
    trading_fee_taker FLOAT,
    disable           BOOLEAN     NOT NULL DEFAULT TRUE
        -- only allow to enable exchange if trading_fee_maker and trading_fee_taker are both set
        CONSTRAINT disable_check CHECK (disable OR
                                        ((trading_fee_maker IS NOT NULL) AND (trading_fee_taker IS NOT NULL)))
);

CREATE TABLE IF NOT EXISTS "addresses"
(
    id      SERIAL PRIMARY KEY,
    address TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS "assets"
(
    id                            SERIAL PRIMARY KEY,
    symbol                        TEXT      NOT NULL UNIQUE,
    name                          TEXT      NOT NULL,
    address_id                    INT       NULL REFERENCES addresses (id)
        CONSTRAINT address_id_check CHECK ( address_id IS NOT NULL OR NOT transferable),
    decimals                      INT       NOT NULL,
    -- transferable must be set to true if it is possible to withdraw/deposit 
    -- to reserve (ETH or ERC20 tokens). If transferable is true, the address and 
    -- deposit address of related asset_exchange records are required.
    transferable                  BOOLEAN   NOT NULL,
    set_rate                      TEXT      NOT NULL,
    rebalance                     BOOLEAN   NOT NULL,
    is_quote                      BOOLEAN   NOT NULL,

    pwi_ask_a                     FLOAT     NULL,
    pwi_ask_b                     FLOAT     NULL,
    pwi_ask_c                     FLOAT     NULL,
    pwi_ask_min_min_spread        FLOAT     NULL,
    pwi_ask_price_multiply_factor FLOAT     NULL,
    pwi_bid_a                     FLOAT     NULL,
    pwi_bid_b                     FLOAT     NULL,
    pwi_bid_c                     FLOAT     NULL,
    pwi_bid_min_min_spread        FLOAT     NULL,
    pwi_bid_price_multiply_factor FLOAT     NULL,

    rebalance_quadratic_a         FLOAT     NULL,
    rebalance_quadratic_b         FLOAT     NULL,
    rebalance_quadratic_c         FLOAT     NULL,

    target_total                  FLOAT     NULL,
    target_reserve                FLOAT     NULL,
    target_rebalance_threshold    FLOAT     NULL,
    target_transfer_threshold     FLOAT     NULL,

    created                       TIMESTAMP NOT NULL,
    updated                       TIMESTAMP NOT NULL,
    -- if set_rate strategy is defined, pwi columns are required
    CONSTRAINT pwi_check CHECK (
            set_rate = 'not_set'
            OR (pwi_ask_a IS NOT NULL AND
                pwi_ask_b IS NOT NULL AND
                pwi_ask_c IS NOT NULL AND
                pwi_ask_min_min_spread IS NOT NULL AND
                pwi_ask_price_multiply_factor IS NOT NULL AND
                pwi_bid_a IS NOT NULL AND
                pwi_bid_b IS NOT NULL AND
                pwi_bid_c IS NOT NULL AND
                pwi_bid_min_min_spread IS NOT NULL AND
                pwi_bid_price_multiply_factor IS NOT NULL
                )),
    -- if rebalance is true, rebalance quadratic is required
    CONSTRAINT rebalance_quadratic_check CHECK (
            NOT rebalance OR
            (rebalance_quadratic_a IS NOT NULL AND
             rebalance_quadratic_b IS NOT NULL AND
             rebalance_quadratic_c IS NOT NULL)),
    -- if rebalance is true, target configuration is required
    CONSTRAINT target_check CHECK (
            NOT rebalance OR
            (target_total IS NOT NULL AND
             target_reserve IS NOT NULL AND
             target_rebalance_threshold IS NOT NULL AND
             target_transfer_threshold IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS "asset_old_addresses"
(
    id         SERIAL PRIMARY KEY,
    address_id INT NOT NULL REFERENCES addresses (id),
    asset_id   INT NOT NULL REFERENCES assets (id)
    -- TODO add a constraint to ensure that asset_id is not linked to any asset in address field already 
);

CREATE TABLE IF NOT EXISTS "asset_exchanges"
(
    id                 SERIAL PRIMARY KEY,
    exchange_id        INT REFERENCES exchanges (id) NOT NULL,
    asset_id           INT REFERENCES assets (id)    NOT NULL,
    symbol             TEXT                          NOT NULL,
    deposit_address    TEXT                          NULL,
    min_deposit        FLOAT                         NOT NULL,
    withdraw_fee       FLOAT                         NOT NULL,
    target_recommended FLOAT                         NOT NULL,
    target_ratio       FLOAT                         NOT NULL,
    UNIQUE (exchange_id, asset_id)
);

CREATE TABLE IF NOT EXISTS trading_pairs
(
    id               SERIAL PRIMARY KEY,
    exchange_id      INT REFERENCES exchanges (id) NOT NULL,
    base_id          INT REFERENCES assets (id)    NOT NULL,
    quote_id         INT REFERENCES assets (id)    NOT NULL,
    price_precision  INT                           NOT NULL,
    amount_precision INT                           NOT NULL,
    amount_limit_min FLOAT                         NOT NULL,
    amount_limit_max FLOAT                         NOT NULL,
    price_limit_min  FLOAT                         NOT NULL,
    price_limit_max  FLOAT                         NOT NULL,
    min_notional     FLOAT                         NOT NULL,
    UNIQUE (exchange_id, base_id, quote_id),
    CONSTRAINT trading_pair_check CHECK ( base_id != quote_id)
);
-- this table manage which asset will be use to buy/sell when trading.
CREATE TABLE IF NOT EXISTS trading_by
(
    id              SERIAL PRIMARY KEY,
    asset_id        INT REFERENCES assets (id)        NOT NULL,
    trading_pair_id INT REFERENCES trading_pairs (id) NOT NULL,
    UNIQUE (asset_id, trading_pair_id)
);

CREATE TABLE IF NOT EXISTS create_assets
(
    id      SERIAL PRIMARY KEY,
    created TIMESTAMP NOT NULL,
    data    JSON      NOT NULL
);

CREATE TABLE IF NOT EXISTS update_exchanges
(
    id      SERIAL PRIMARY KEY,
    created TIMESTAMP NOT NULL,
    data    JSON      NOT NULL
);

CREATE TABLE IF NOT EXISTS create_asset_exchanges
(
    id      SERIAL PRIMARY KEY,
    created TIMESTAMP NOT NULL,
    data    JSON      NOT NULL
);

CREATE TABLE IF NOT EXISTS update_asset_exchanges
(
    id      SERIAL PRIMARY KEY,
    created TIMESTAMP NOT NULL,
    data    JSON      NOT NULL
);

CREATE TABLE IF NOT EXISTS update_assets
(
    id      SERIAL PRIMARY KEY,
    created TIMESTAMP NOT NULL,
    data    JSON      NOT NULL
);

CREATE TABLE IF NOT EXISTS create_trading_pairs
(
    id      SERIAL PRIMARY KEY,
    created TIMESTAMP NOT NULL,
    data    JSON      NOT NULL
);
CREATE TABLE IF NOT EXISTS update_trading_pairs
(
    id      SERIAL PRIMARY KEY,
    created TIMESTAMP NOT NULL,
    data    JSON      NOT NULL
);

CREATE OR REPLACE FUNCTION new_create_asset(_data create_assets.data%TYPE)
    RETURNS int AS
$$

DECLARE
    _id create_assets.id%TYPE;

BEGIN
    DELETE FROM create_assets;
    INSERT INTO create_assets(created, data) VALUES (now(), _data) RETURNING id INTO _id;
    RETURN _id;
END

$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_update_asset(_data update_assets.data%TYPE)
    RETURNS int AS
$$
DECLARE
    _id update_assets.id%TYPE;
BEGIN
    DELETE FROM update_assets;
    INSERT INTO update_assets(created, data) VALUES (now(), _data) RETURNING id into _id;
    RETURN _id;
END;
$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_update_exchange(_data update_exchanges.data%TYPE)
    RETURNS int AS
$$
DECLARE
    _id update_exchanges.id%TYPE;
BEGIN
    DELETE FROM update_exchanges;
    INSERT INTO update_exchanges(created, data) VALUES (now(), _data) RETURNING id into _id;
    RETURN _id;
END;

$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_create_asset_exchange(_data create_asset_exchanges.data%TYPE)
    RETURNS int AS
$$

DECLARE
    _id create_asset_exchanges.id%TYPE;

BEGIN
    DELETE FROM create_asset_exchanges;
    INSERT INTO create_asset_exchanges(created, data) VALUES (now(), _data) RETURNING id INTO _id;
    RETURN _id;
END

$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_update_asset_exchange(_data update_asset_exchanges.data%TYPE)
    RETURNS int AS
$$

DECLARE
    _id update_asset_exchanges.id%TYPE;

BEGIN
    DELETE FROM update_asset_exchanges;
    INSERT INTO update_asset_exchanges(created, data) VALUES (now(), _data) RETURNING id INTO _id;
    RETURN _id;
END

$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_create_trading_pair(_data create_trading_pairs.data%TYPE)
    RETURNS int AS
$$
DECLARE
    _id create_trading_pairs.id%TYPE;
BEGIN
    DELETE FROM create_trading_pairs;
    INSERT INTO create_trading_pairs(created, data) VALUES (now(), _data) RETURNING id into _id;
    RETURN _id;
END;
$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_update_trading_pair(_data update_trading_pairs.data%TYPE)
    RETURNS int AS
$$
DECLARE
    _id update_trading_pairs.id%TYPE;
BEGIN
    DELETE FROM update_trading_pairs;
    INSERT INTO update_trading_pairs(created, data) VALUES (now(), _data) RETURNING id into _id;
    RETURN _id;
END;
$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_asset(_symbol assets.symbol%TYPE,
                                     _name assets.symbol%TYPE,
                                     _address addresses.address%TYPE,
                                     _decimals assets.decimals%TYPE,
                                     _transferable assets.transferable%TYPE,
                                     _set_rate assets.set_rate%TYPE,
                                     _rebalance assets.rebalance%TYPE,
                                     _is_quote assets.is_quote%TYPE,
                                     _pwi_ask_a assets.pwi_ask_a%TYPE,
                                     _pwi_ask_b assets.pwi_ask_b%TYPE,
                                     _pwi_ask_c assets.pwi_ask_c%TYPE,
                                     _pwi_ask_min_min_spread assets.pwi_ask_min_min_spread%TYPE,
                                     _pwi_ask_price_multiply_factor assets.pwi_ask_price_multiply_factor%TYPE,
                                     _pwi_bid_a assets.pwi_bid_a%TYPE,
                                     _pwi_bid_b assets.pwi_bid_b%TYPE,
                                     _pwi_bid_c assets.pwi_bid_c%TYPE,
                                     _pwi_bid_min_min_spread assets.pwi_bid_min_min_spread%TYPE,
                                     _pwi_bid_price_multiply_factor assets.pwi_bid_price_multiply_factor%TYPE,
                                     _rebalance_quadratic_a assets.rebalance_quadratic_a%TYPE,
                                     _rebalance_quadratic_b assets.rebalance_quadratic_b%TYPE,
                                     _rebalance_quadratic_c assets.rebalance_quadratic_c%TYPE,
                                     _target_total assets.target_total%TYPE,
                                     _target_reserve assets.target_reserve%TYPE,
                                     _target_rebalance_threshold assets.target_rebalance_threshold%TYPE,
                                     _target_transfer_threshold assets.target_total%TYPE)
    RETURNS int AS
$$
DECLARE
    _address_id addresses.id%TYPE;
    _id         assets.id%TYPE;
BEGIN
    IF _address IS NOT NULL THEN
        INSERT INTO "addresses" (address) VALUES (_address) RETURNING id INTO _address_id;
    END IF;

    INSERT
    INTO assets(symbol,
                name,
                address_id,
                decimals,
                transferable,
                set_rate,
                rebalance,
                is_quote,
                pwi_ask_a,
                pwi_ask_b,
                pwi_ask_c,
                pwi_ask_min_min_spread,
                pwi_ask_price_multiply_factor,
                pwi_bid_a,
                pwi_bid_b,
                pwi_bid_c,
                pwi_bid_min_min_spread,
                pwi_bid_price_multiply_factor,
                rebalance_quadratic_a,
                rebalance_quadratic_b,
                rebalance_quadratic_c,
                target_total,
                target_reserve,
                target_rebalance_threshold,
                target_transfer_threshold,
                created,
                updated)
    VALUES (_symbol,
            _name,
            _address_id,
            _decimals,
            _transferable,
            _set_rate,
            _rebalance,
            _is_quote,
            _pwi_ask_a,
            _pwi_ask_b,
            _pwi_ask_c,
            _pwi_ask_min_min_spread,
            _pwi_ask_price_multiply_factor,
            _pwi_bid_a,
            _pwi_bid_b,
            _pwi_bid_c,
            _pwi_bid_min_min_spread,
            _pwi_bid_price_multiply_factor,
            _rebalance_quadratic_a,
            _rebalance_quadratic_b,
            _rebalance_quadratic_c,
            _target_total,
            _target_reserve,
            _target_rebalance_threshold,
            _target_transfer_threshold,
            now(),
            now()) RETURNING id INTO _id;

    RETURN _id;
END
$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION change_asset_address(_id assets.id%TYPE, _address addresses.address%TYPE) RETURNS VOID AS
$$
DECLARE
    _new_address_id addresses.id%TYPE;
BEGIN
    PERFORM id FROM assets WHERE id = _id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'asset with id % does not exists', _id USING ERRCODE = 'assert_failure';
    END IF;

    INSERT INTO "asset_old_addresses" (address_id, asset_id)
    SELECT addresses.id, assets.id
    FROM assets
             LEFT JOIN addresses ON assets.address_id = addresses.id
    WHERE assets.id = _id;

    INSERT INTO "addresses" (address) VALUES (_address) RETURNING id INTO _new_address_id;

    UPDATE "assets"
    SET address_id = _new_address_id,
        updated    = now()
    WHERE assets.id = _id;
    RETURN;
END
$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_trading_pair(_exchange_id trading_pairs.exchange_id%TYPE,
                                            _base_id trading_pairs.base_id%TYPE,
                                            _quote_id trading_pairs.quote_id%TYPE,
                                            _price_precision trading_pairs.price_precision%TYPE,
                                            _amount_precision trading_pairs.amount_precision%TYPE,
                                            _amount_limit_min trading_pairs.amount_limit_min%TYPE,
                                            _amount_limit_max trading_pairs.amount_limit_max%TYPE,
                                            _price_limit_min trading_pairs.price_limit_min%TYPE,
                                            _price_limit_max trading_pairs.price_limit_max%TYPE,
                                            _min_notional trading_pairs.min_notional%TYPE)
    RETURNS INT AS
$$
DECLARE
    _id                   trading_pairs.id%TYPE;
    _quote_asset_is_quote assets.is_quote%TYPE;
BEGIN
    PERFORM id FROM asset_exchanges WHERE exchange_id = _exchange_id AND asset_id = _base_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'base asset is not configured for exchange base_id=% exchange_id=%',
            _base_id,_exchange_id USING ERRCODE = 'assert_failure';
    END IF;

    PERFORM id FROM asset_exchanges WHERE exchange_id = _exchange_id AND asset_id = _quote_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'quote asset is not configured for exchange quote_id=% exchange_id=%',
            _quote_id,_exchange_id USING ERRCODE = 'assert_failure';
    END IF;

    PERFORM id FROM assets WHERE id = _base_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'base asset is not found base_id=%', _base_id USING ERRCODE = 'assert_failure';
    END IF;

    SELECT is_quote FROM assets WHERE id = _quote_id INTO _quote_asset_is_quote;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'quote asset is not found quote_id=%', _quote_id USING ERRCODE = 'assert_failure';
    END IF;

    IF NOT _quote_asset_is_quote THEN
        RAISE EXCEPTION 'quote asset is not configured as quote id=%', _quote_id USING ERRCODE = 'assert_failure';
    END IF;

    INSERT INTO trading_pairs (exchange_id,
                               base_id,
                               quote_id,
                               price_precision,
                               amount_precision,
                               amount_limit_min,
                               amount_limit_max,
                               price_limit_min,
                               price_limit_max,
                               min_notional)
    VALUES (_exchange_id,
            _base_id,
            _quote_id,
            _price_precision,
            _amount_precision,
            _amount_limit_min,
            _amount_limit_max,
            _price_limit_min,
            _price_limit_max,
            _min_notional) RETURNING id INTO _id;
    RETURN _id;
END
$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION new_trading_by(_asset_id assets.id%TYPE,
                                            _trading_pair_id trading_pairs.id%TYPE)
    RETURNS INT AS
$$
DECLARE
    _id                   trading_by.id%TYPE;
BEGIN
    PERFORM id FROM trading_pairs WHERE base_id = _asset_id OR quote_id = _asset_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'asset must be base or quote in trading pair, asset=%',
            _asset_id USING ERRCODE = 'assert_failure';
    END IF;

    INSERT INTO trading_by (asset_id, trading_pair_id)
    VALUES (_asset_id,_trading_pair_id) RETURNING id INTO _id;
    RETURN _id;
END
$$ LANGUAGE PLPGSQL;
`

type preparedStmts struct {
	getExchanges               *sqlx.Stmt
	getExchange                *sqlx.Stmt
	getExchangeByName          *sqlx.Stmt
	updateExchange             *sqlx.NamedStmt
	newAsset                   *sqlx.NamedStmt
	newAssetExchange           *sqlx.NamedStmt
	updateAssetExchange        *sqlx.NamedStmt
	newCreateAssetExchange     *sqlx.Stmt
	getCreateAssetExchanges    *sqlx.Stmt
	deletePendingAssetExchange *sqlx.Stmt

	newTradingBy *sqlx.Stmt
	getTradingBy *sqlx.Stmt

	newUpdateAssetExchange    *sqlx.Stmt
	getUpdateAssetExchanges   *sqlx.Stmt
	deleteUpdateAssetExchange *sqlx.Stmt

	newTradingPair *sqlx.NamedStmt

	getAsset                 *sqlx.Stmt
	getAssetBySymbol         *sqlx.Stmt
	getAssetExchange         *sqlx.Stmt
	getAssetExchangeBySymbol *sqlx.Stmt
	getTradingPair           *sqlx.Stmt
	updateAsset              *sqlx.NamedStmt
	changeAssetAddress       *sqlx.Stmt
	updateDepositAddress     *sqlx.Stmt
	updateTradingPair        *sqlx.NamedStmt

	getTradingPairByID    *sqlx.Stmt
	getTradingPairSymbols *sqlx.Stmt
	getMinNotional        *sqlx.Stmt
	// getTransferableAssets *sqlx.Stmt
	getCreateAssets   *sqlx.Stmt
	newCreateAsset    *sqlx.Stmt
	deleteCreateAsset *sqlx.Stmt

	newUpdateAsset    *sqlx.Stmt
	getUpdateAssets   *sqlx.Stmt
	deleteUpdateAsset *sqlx.Stmt

	newUpdateExchange    *sqlx.Stmt
	getUpdateExchanges   *sqlx.Stmt
	deleteUpdateExchange *sqlx.Stmt

	newCreateTradingPair    *sqlx.Stmt
	getCreateTradingPairs   *sqlx.Stmt
	deleteCreateTradingPair *sqlx.Stmt

	newUpdateTradingPair    *sqlx.Stmt
	getUpdateTradingPairs   *sqlx.Stmt
	deleteUpdateTradingPair *sqlx.Stmt
}

func newPreparedStmts(db *sqlx.DB) (*preparedStmts, error) {
	getExchanges, getExchange, getExchangeByName, updateExchange, err := exchangeStatements(db)
	if err != nil {
		return nil, err
	}

	newAsset, getAsset, updateAsset, getAssetBySymbol, err := assetStatements(db)
	if err != nil {
		return nil, err
	}

	newAssetExchange, updateAssetExchange, getAssetExchange, getAssetExchangeBySymbol, err := assetExchangeStatements(db)
	if err != nil {
		return nil, err
	}

	newCreateAsset, deleteCreateAsset, getCreateAsset, err := createAssetStatements(db)
	if err != nil {
		return nil, err
	}

	newUpdateAsset, deleteUpdateAsset, listUpdateAsset, err := updateAssetStatements(db)
	if err != nil {
		return nil, err
	}

	newUpdateExchange, deleteUpdateExchange, listUpdateExchange, err := updateExchangeStatements(db)
	if err != nil {
		return nil, err
	}

	newCreateTradingPair, deleteCreateTradingPair, listCreateTradingPair, err := createTradingPairStatements(db)
	if err != nil {
		return nil, err
	}

	newUpdateTradingPair, deleteUpdateTradingPair, listUpdateTradingPair, err := updateTradingPairStatements(db)
	if err != nil {
		return nil, err
	}

	newTradingPair, getTradingPair, updateTradingPair, getTradingPairByID, getTradingPairSymbols, err := tradingPairStatements(db)
	if err != nil {
		return nil, err
	}

	const changeAssetAddressQuery = `SELECT change_asset_address($1, $2);`
	changeAssetAddress, err := db.Preparex(changeAssetAddressQuery)
	if err != nil {
		return nil, err
	}

	const getMinNotionalQuery = `SELECT min_notional
									FROM trading_pairs
									WHERE exchange_id = $1
									  AND base_id = $2
									  AND quote_id = $3;
									`
	getMinNotional, err := db.Preparex(getMinNotionalQuery)
	if err != nil {
		return nil, err
	}

	const updateDepositAddressQuery = `UPDATE asset_exchanges
									SET deposit_address = $3
									WHERE asset_id = $1
									  AND exchange_id = $2 RETURNING id;`
	updateDepositAddress, err := db.Preparex(updateDepositAddressQuery)
	if err != nil {
		return nil, err
	}

	newCreateAssetExchanges, getPendingAssetExchanges, deleteCreateAssetExchange, err := createAssetExchangeStatements(db)
	if err != nil {
		return nil, err
	}

	newUpdateAssetExchanges, getUpdateAssetExchanges, deleteUpdateAssetExchange, err := updateAssetExchangeStatements(db)
	if err != nil {
		return nil, err
	}

	newTradingBy, getTradingBy, err := tradingByStatements(db)
	if err != nil {
		return nil, err
	}

	return &preparedStmts{
		getExchanges:        getExchanges,
		getExchange:         getExchange,
		getExchangeByName:   getExchangeByName,
		updateExchange:      updateExchange,
		newAsset:            newAsset,
		newAssetExchange:    newAssetExchange,
		updateAssetExchange: updateAssetExchange,

		newCreateAssetExchange:     newCreateAssetExchanges,
		getCreateAssetExchanges:    getPendingAssetExchanges,
		deletePendingAssetExchange: deleteCreateAssetExchange,

		newUpdateAssetExchange:    newUpdateAssetExchanges,
		getUpdateAssetExchanges:   getUpdateAssetExchanges,
		deleteUpdateAssetExchange: deleteUpdateAssetExchange,

		newTradingPair: newTradingPair,
		newTradingBy:   newTradingBy,
		getTradingBy:   getTradingBy,

		getAsset:                 getAsset,
		getAssetBySymbol:         getAssetBySymbol,
		getAssetExchange:         getAssetExchange,
		getAssetExchangeBySymbol: getAssetExchangeBySymbol,
		getTradingPair:           getTradingPair,
		updateAsset:              updateAsset,
		changeAssetAddress:       changeAssetAddress,
		updateDepositAddress:     updateDepositAddress,
		updateTradingPair:        updateTradingPair,

		getTradingPairByID:    getTradingPairByID,
		getTradingPairSymbols: getTradingPairSymbols,
		getMinNotional:        getMinNotional,

		newCreateAsset:    newCreateAsset,
		getCreateAssets:   getCreateAsset,
		deleteCreateAsset: deleteCreateAsset,

		newUpdateAsset:    newUpdateAsset,
		getUpdateAssets:   listUpdateAsset,
		deleteUpdateAsset: deleteUpdateAsset,

		newUpdateExchange:    newUpdateExchange,
		getUpdateExchanges:   listUpdateExchange,
		deleteUpdateExchange: deleteUpdateExchange,

		newCreateTradingPair:    newCreateTradingPair,
		getCreateTradingPairs:   listCreateTradingPair,
		deleteCreateTradingPair: deleteCreateTradingPair,

		newUpdateTradingPair:    newUpdateTradingPair,
		getUpdateTradingPairs:   listUpdateTradingPair,
		deleteUpdateTradingPair: deleteUpdateTradingPair,
	}, nil
}

func tradingPairStatements(db *sqlx.DB) (*sqlx.NamedStmt, *sqlx.Stmt, *sqlx.NamedStmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	const newTradingPairQuery = `SELECT new_trading_pair
									FROM new_trading_pair(:exchange_id,
									                      :base_id,
									                      :quote_id,
									                      :price_precision,
									                      :amount_precision,
									                      :amount_limit_min,
									                      :amount_limit_max,
									                      :price_limit_min,
									                      :price_limit_max,
									                      :min_notional);`
	newTradingPair, err := db.PrepareNamed(newTradingPairQuery)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrap(err, "failed to prepare newTradingPair")
	}
	const getTradingPairQuery = `SELECT DISTINCT tp.id,
									                tp.exchange_id,
									                tp.base_id,
									                tp.quote_id,
									                tp.price_precision,
									                tp.amount_precision,
									                tp.amount_limit_min,
									                tp.amount_limit_max,
									                tp.price_limit_min,
									                tp.price_limit_max,
									                tp.min_notional
									FROM trading_pairs tp
									         INNER JOIN asset_exchanges ae ON tp.exchange_id = ae.exchange_id
									WHERE ae.asset_id = coalesce($1, ae.asset_id);
									`
	getTradingPair, err := db.Preparex(getTradingPairQuery)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrap(err, "failed to prepare getTradingPair")
	}
	const updateTradingPairQuery = `UPDATE "trading_pairs"
									SET price_precision  = coalesce(:price_precision, price_precision),
									    amount_precision = coalesce(:amount_precision, amount_precision),
									    amount_limit_min = coalesce(:amount_limit_min, amount_limit_min),
									    amount_limit_max = coalesce(:amount_limit_max, amount_limit_max),
									    price_limit_min  = coalesce(:price_limit_min, price_limit_min),
									    price_limit_max  = coalesce(:price_limit_max, price_limit_max),
									    min_notional= coalesce(:min_notional, min_notional)
									WHERE id = :id RETURNING id; `
	updateTradingPair, err := db.PrepareNamed(updateTradingPairQuery)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrap(err, "failed to updateTradingPair")
	}

	const getTradingPairByIDQuery = `SELECT DISTINCT tp.id,
									                tp.exchange_id,
									                tp.base_id,
									                tp.quote_id,
									                tp.price_precision,
									                tp.amount_precision,
									                tp.amount_limit_min,
									                tp.amount_limit_max,
									                tp.price_limit_min,
									                tp.price_limit_max,
									                tp.min_notional,
									                bae.symbol AS base_symbol,
									                qae.symbol AS quote_symbol
									FROM trading_pairs AS tp
									         INNER JOIN assets AS ba ON tp.base_id = ba.id
									         INNER JOIN asset_exchanges AS bae ON ba.id = bae.asset_id
									         INNER JOIN assets AS qa ON tp.quote_id = qa.id
									         INNER JOIN asset_exchanges AS qae ON qa.id = qae.asset_id
									WHERE tp.exchange_id = bae.exchange_id AND tp.exchange_id = qae.exchange_id AND tp.id = $1;`
	getTradingPairByID, err := db.Preparex(getTradingPairByIDQuery)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrap(err, "failed to prepare getTradingPairByID")
	}

	const getTradingPairSymbolsQuery = `SELECT DISTINCT tp.id,
									                tp.exchange_id,
									                tp.base_id,
									                tp.quote_id,
									                tp.price_precision,
									                tp.amount_precision,
									                tp.amount_limit_min,
									                tp.amount_limit_max,
									                tp.price_limit_min,
									                tp.price_limit_max,
									                tp.min_notional,
									                bae.symbol AS base_symbol,
									                qae.symbol AS quote_symbol
									FROM trading_pairs AS tp
									         INNER JOIN assets AS ba ON tp.base_id = ba.id
									         INNER JOIN asset_exchanges AS bae ON ba.id = bae.asset_id
									         INNER JOIN assets AS qa ON tp.quote_id = qa.id
									         INNER JOIN asset_exchanges AS qae ON qa.id = qae.asset_id
									WHERE tp.exchange_id = $1;`
	getTradingPairSymbols, err := db.Preparex(getTradingPairSymbolsQuery)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrap(err, "failed to prepare getTradingPairSymbols")
	}

	return newTradingPair, getTradingPair, updateTradingPair, getTradingPairByID, getTradingPairSymbols, nil
}

func assetStatements(db *sqlx.DB) (*sqlx.NamedStmt, *sqlx.Stmt, *sqlx.NamedStmt, *sqlx.Stmt, error) {
	const newAssetQuery = `SELECT new_asset
		FROM new_asset(
		             :symbol,
		             :name,
		             :address,
		             :decimals,
		             :transferable,
		             :set_rate,
		             :rebalance,
		             :is_quote,
		             :ask_a,
		             :ask_b,
		             :ask_c,
		             :ask_min_min_spread,
		             :ask_price_multiply_factor,
		             :bid_a,
		             :bid_b,
		             :bid_c,
		             :bid_min_min_spread,
		             :bid_price_multiply_factor,
		             :rebalance_quadratic_a,
		             :rebalance_quadratic_b,
		             :rebalance_quadratic_c,
		             :target_total,
		             :target_reserve,
		             :target_rebalance_threshold,
		             :target_transfer_threshold
		         );`
	newAsset, err := db.PrepareNamed(newAssetQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare newAsset")
	}
	const getAssetQuery = `SELECT assets.id,
								       assets.symbol,
								       assets.name,
								       a.address,
								       array_agg(oa.address) FILTER ( WHERE oa.address IS NOT NULL ) AS old_addresses,
								       assets.decimals,
								       assets.transferable,
								       assets.set_rate,
								       assets.rebalance,
								       assets.is_quote,
								       assets.pwi_ask_a,
								       assets.pwi_ask_b,
								       assets.pwi_ask_c,
								       assets.pwi_ask_min_min_spread,
								       assets.pwi_ask_price_multiply_factor,
								       assets.pwi_bid_a,
								       assets.pwi_bid_b,
								       assets.pwi_bid_c,
								       assets.pwi_bid_min_min_spread,
								       assets.pwi_bid_price_multiply_factor,
								       assets.rebalance_quadratic_a,
								       assets.rebalance_quadratic_b,
								       assets.rebalance_quadratic_c,
								       assets.target_total,
								       assets.target_reserve,
								       assets.target_rebalance_threshold,
								       assets.target_transfer_threshold,
								       assets.created,
								       assets.updated
								FROM assets
								         LEFT JOIN addresses a on assets.address_id = a.id
								         LEFT JOIN asset_old_addresses aoa on assets.id = aoa.asset_id
								         LEFT JOIN addresses oa ON aoa.address_id = oa.id
								WHERE assets.id = coalesce($1, assets.id)
								  AND assets.transferable = coalesce($2, assets.transferable)
								GROUP BY assets.id,
								         assets.symbol,
								         assets.name,
								         a.address,
								         assets.decimals,
								         assets.transferable,
								         assets.set_rate,
								         assets.rebalance,
								         assets.is_quote,
								         assets.pwi_ask_a,
								         assets.pwi_ask_b,
								         assets.pwi_ask_c,
								         assets.pwi_ask_min_min_spread,
								         assets.pwi_ask_price_multiply_factor,
								         assets.pwi_bid_a,
								         assets.pwi_bid_b,
								         assets.pwi_bid_c,
								         assets.pwi_bid_min_min_spread,
								         assets.pwi_bid_price_multiply_factor,
								         assets.rebalance_quadratic_a,
								         assets.rebalance_quadratic_b,
								         assets.rebalance_quadratic_c,
								         assets.target_total,
								         assets.target_reserve,
								         assets.target_rebalance_threshold,
								         assets.target_transfer_threshold,
								         assets.created,
								         assets.updated
								ORDER BY assets.id`
	getAsset, err := db.Preparex(getAssetQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare getAsset")
	}

	getAssetBySymbolQuery := `SELECT id, decimals FROM assets WHERE symbol = $1`
	getAssetBySymbol, err := db.Preparex(getAssetBySymbolQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare getAssetBySymbolQuery")
	}

	const updateAssetQuery = `WITH updated AS (
								    UPDATE "addresses"
								        SET address = COALESCE(:address, addresses.address)
								        FROM "assets"
								        WHERE assets.id = :id AND assets.address_id = addresses.id
									)
								UPDATE "assets"
								SET symbol       = COALESCE(:symbol, symbol),
								    name         = COALESCE(:name, name),
								    decimals     = COALESCE(:decimals, decimals),
								    transferable = COALESCE(:transferable, transferable),
								    set_rate     = COALESCE(:set_rate, set_rate),
								    rebalance    = COALESCE(:rebalance, rebalance),
								    is_quote     = COALESCE(:is_quote, is_quote),
								    pwi_ask_a = COALESCE(:ask_a,pwi_ask_a),
									pwi_ask_b = COALESCE(:ask_b, pwi_ask_b),
									pwi_ask_c = COALESCE(:ask_c, pwi_ask_c),
									pwi_ask_min_min_spread = COALESCE(:ask_min_min_spread,pwi_ask_min_min_spread),
									pwi_ask_price_multiply_factor = COALESCE(:ask_price_multiply_factor, pwi_ask_price_multiply_factor),
									pwi_bid_a = COALESCE(:bid_a,pwi_bid_a),
									pwi_bid_b = COALESCE(:bid_b,pwi_bid_b),
									pwi_bid_c = COALESCE(:bid_c,pwi_bid_c),
									pwi_bid_min_min_spread = COALESCE(:bid_min_min_spread,pwi_bid_min_min_spread),
									pwi_bid_price_multiply_factor = COALESCE(:bid_price_multiply_factor,pwi_bid_price_multiply_factor),
									rebalance_quadratic_a = COALESCE(:rebalance_quadratic_a,rebalance_quadratic_a),
									rebalance_quadratic_b = COALESCE(:rebalance_quadratic_b,rebalance_quadratic_b),
									rebalance_quadratic_c = COALESCE(:rebalance_quadratic_c,rebalance_quadratic_c),
									target_total = COALESCE(:target_total,target_total),
									target_reserve = COALESCE(:target_total,target_reserve),
									target_rebalance_threshold = COALESCE(:target_total,target_rebalance_threshold),
									target_transfer_threshold = COALESCE(:target_total,target_transfer_threshold),
								    updated      = now()
								WHERE id = :id RETURNING id;
								`
	updateAsset, err := db.PrepareNamed(updateAssetQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare updateAsset")
	}
	return newAsset, getAsset, updateAsset, getAssetBySymbol, nil
}

func assetExchangeStatements(db *sqlx.DB) (*sqlx.NamedStmt, *sqlx.NamedStmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	const newAssetExchangeQuery string = `INSERT INTO asset_exchanges(exchange_id,
		                            asset_id,
		                            symbol,
		                            deposit_address,
		                            min_deposit,
		                            withdraw_fee,
		                            target_recommended,
		                            target_ratio)
		VALUES (:exchange_id,
		        :asset_id,
		        :symbol,
		        :deposit_address,
		        :min_deposit,
		        :withdraw_fee,
		        :target_recommended,
		        :target_ratio) RETURNING id`
	newAssetExchange, err := db.PrepareNamed(newAssetExchangeQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare newAssetExchange")
	}
	const updateAssetExchangeQuery string = `UPDATE "asset_exchanges"
		SET symbol = COALESCE(:symbol, symbol),
		    deposit_address = COALESCE(:deposit_address, deposit_address),
		    min_deposit           = COALESCE(:min_deposit, min_deposit),
			withdraw_fee = coalesce(:withdraw_fee, withdraw_fee),
		    target_recommended = coalesce(:target_recommended,target_recommended),
		    target_ratio = coalesce(:target_ratio, target_ratio)
		WHERE id = :id RETURNING id;`
	updateAssetExchange, err := db.PrepareNamed(updateAssetExchangeQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare updateAssetExchange")
	}

	const getAssetExchangeQuery = `SELECT id,
			       exchange_id,
			       asset_id,
			       symbol,
			       deposit_address,
			       min_deposit,
			       withdraw_fee,
			       target_recommended,
			       target_ratio
			FROM asset_exchanges
			WHERE asset_id = coalesce($1, asset_id)
			AND id = coalesce($2, id)`
	getAssetExchange, err := db.Preparex(getAssetExchangeQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare getAssetExchange")
	}

	const getAssetExchangeBySymbolQuery = `SELECT
		asset_exchanges.asset_id as id,
		asset_exchanges.symbol as symbol,
		a.decimals as decimals	
	FROM asset_exchanges
		LEFT JOIN assets a on asset_exchanges.asset_id = a.id
	WHERE asset_exchanges.exchange_id = $1
	AND asset_exchanges.symbol= $2`
	getAssetExchangeBySymbol, err := db.Preparex(getAssetExchangeBySymbolQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare getAssetExchangeBySymbol")
	}

	return newAssetExchange, updateAssetExchange, getAssetExchange, getAssetExchangeBySymbol, nil
}

func exchangeStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, *sqlx.Stmt, *sqlx.NamedStmt, error) {
	const getExchangesQuery = `SELECT * FROM "exchanges";`
	getExchanges, err := db.Preparex(getExchangesQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare get exchanges")
	}
	const getExchangeQuery = `SELECT * FROM "exchanges" WHERE id = $1`
	getExchange, err := db.Preparex(getExchangeQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare get exchange")
	}
	const getExchangeByNameQuery = `SELECT * FROM "exchanges" WHERE name = $1`
	getExchangeByName, err := db.Preparex(getExchangeByNameQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare get exchange by name")
	}
	const updateExchangeQuery = `UPDATE "exchanges"
	SET trading_fee_maker = COALESCE(:trading_fee_maker, trading_fee_maker),
	    trading_fee_taker = COALESCE(:trading_fee_taker, trading_fee_taker),
	    disable           = COALESCE(:disable, disable)
	WHERE id = :id
	`
	updateExchange, err := db.PrepareNamed(updateExchangeQuery)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to prepare update exchange")
	}
	return getExchanges, getExchange, getExchangeByName, updateExchange, nil
}

func createAssetExchangeStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	const newCreateAssetExchangesQuery = `SELECT new_create_asset_exchange FROM new_create_asset_exchange ($1);`
	newCreateAssetExchanges, err := db.Preparex(newCreateAssetExchangesQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare newCreateAssetExchanges")
	}
	const getCreateAssetExchangeQuery = `SELECT id,created,data FROM create_asset_exchanges WHERE id=COALESCE($1, create_asset_exchanges.id)`
	getPendingAssetExchanges, err := db.Preparex(getCreateAssetExchangeQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare getPendingAssetExchanges")
	}
	const deleteCreateAssetExchangeQuery = `DELETE FROM create_asset_exchanges WHERE id=$1`
	deleteCreateAssetExchange, err := db.Preparex(deleteCreateAssetExchangeQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare deleteCreateAssetExchange")
	}
	return newCreateAssetExchanges, getPendingAssetExchanges, deleteCreateAssetExchange, nil
}

func updateAssetExchangeStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	const newUpdateAssetExchangesQuery = `SELECT new_update_asset_exchange FROM new_update_asset_exchange ($1);`
	newUpdateAssetExchanges, err := db.Preparex(newUpdateAssetExchangesQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare newUpdateAssetExchange")
	}
	const listUpdateAssetExchangeQuery = `SELECT id,created,data FROM update_asset_exchanges WHERE id=COALESCE($1, update_asset_exchanges.id)`
	getUpdateAssetExchanges, err := db.Preparex(listUpdateAssetExchangeQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare getUpdateAssetExchanges")
	}
	const deleteUpdateAssetExchangeQuery = `DELETE FROM update_asset_exchanges WHERE id=$1`
	deleteUpdateAssetExchange, err := db.Preparex(deleteUpdateAssetExchangeQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepapre deletUpdateAssetExchange")
	}
	return newUpdateAssetExchanges, getUpdateAssetExchanges, deleteUpdateAssetExchange, nil
}

func updateTradingPairStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	const newUpdateTradingPairQuery = `SELECT new_update_trading_pair FROM new_update_trading_pair($1)`
	newUpdateTradingPair, err := db.Preparex(newUpdateTradingPairQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepapre newUpdateTradingPair")
	}
	const deleteUpdateTradingPairQuery = `DELETE FROM update_trading_pairs WHERE id=$1`
	deleteUpdateTradingPair, err := db.Preparex(deleteUpdateTradingPairQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare deleteUpateTradingPair")
	}
	const listUpdateTradingPairQuery = `SELECT id,created,data FROM update_trading_pairs WHERE id=COALESCE($1, update_trading_pairs.id)`
	listUpdateTradingPair, err := db.Preparex(listUpdateTradingPairQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to listUpdateTradingPair")
	}
	return newUpdateTradingPair, deleteUpdateTradingPair, listUpdateTradingPair, nil
}

func createTradingPairStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	const newCreateTradingPairQuery = `SELECT new_create_trading_pair FROM new_create_trading_pair($1)`
	newCreateTradingPair, err := db.Preparex(newCreateTradingPairQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare newCreateTradingPair")
	}
	const deleteCreateTradingPairQuery = `DELETE FROM create_trading_pairs WHERE id=$1`
	deleteCreateTradingPair, err := db.Preparex(deleteCreateTradingPairQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepapre deleteCreateTradingPair")
	}
	const listCreateTradingPairQuery = `SELECT id,created,data FROM create_trading_pairs WHERE id=COALESCE($1, create_trading_pairs.id)`
	listCreateTradingPair, err := db.Preparex(listCreateTradingPairQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare listCreateTradingPair")
	}
	return newCreateTradingPair, deleteCreateTradingPair, listCreateTradingPair, nil
}

func updateExchangeStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	const newUpdateExchangeQuery = `SELECT new_update_exchange FROM new_update_exchange($1)`
	newUpdateExchange, err := db.Preparex(newUpdateExchangeQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare newUpdateExchange")
	}
	const deleteUpdateExchangeQuery = `DELETE FROM update_exchanges WHERE id=$1`
	deleteUpdateExchange, err := db.Preparex(deleteUpdateExchangeQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare deleteUpdateExchange")
	}
	const listUpdateExchangeQuery = `SELECT id,created,data FROM update_exchanges WHERE id=COALESCE($1, update_exchanges.id)`
	listUpdateExchange, err := db.Preparex(listUpdateExchangeQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare listUpdateExchange")
	}
	return newUpdateExchange, deleteUpdateExchange, listUpdateExchange, nil
}

func updateAssetStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	const newUpdateAssetQuery = `SELECT new_update_asset FROM new_update_asset($1)`
	newUpdateAsset, err := db.Preparex(newUpdateAssetQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare newUpdateAsset")
	}
	const deleteUpdateAssetQuery = `DELETE FROM update_assets WHERE id=$1`
	deleteUpdateAsset, err := db.Preparex(deleteUpdateAssetQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare deleteUpdateAsset")
	}
	const listUpdateAssetQuery = `SELECT id,created,data FROM update_assets WHERE id=COALESCE($1, update_assets.id)`
	listUpdateAsset, err := db.Preparex(listUpdateAssetQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare listUpdateAsset")
	}
	return newUpdateAsset, deleteUpdateAsset, listUpdateAsset, nil
}

func createAssetStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, *sqlx.Stmt, error) {
	// we have at most one pending at a time, so we delete all exists one before insert new one.
	const newCreateAssetQuery = `SELECT new_create_asset FROM new_create_asset ($1);`
	newCreateAsset, err := db.Preparex(newCreateAssetQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to create newCreateAsset")
	}
	const deleteCreateAssetQuery = `DELETE FROM create_assets WHERE id=$1`
	deleteCreateAsset, err := db.Preparex(deleteCreateAssetQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare deleteCreateAsset query")
	}
	const listCreateAssetQuery = `SELECT id,created,data FROM create_assets WHERE id=COALESCE($1, create_assets.id)`
	getCreateAsset, err := db.Preparex(listCreateAssetQuery)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to prepare getCreateAsset")
	}
	return newCreateAsset, deleteCreateAsset, getCreateAsset, nil
}

func tradingByStatements(db *sqlx.DB) (*sqlx.Stmt, *sqlx.Stmt, error) {
	const createTradingByQuery = `SELECT new_trading_by FROM new_trading_by($1,$2);`
	tradingBy, err := db.Preparex(createTradingByQuery)
	if err != nil {
		return nil, nil, err
	}

	const getTradingByQuery = `SELECT id,asset_id,trading_pair_id FROM trading_by WHERE id=COALESCE($1,trading_by.id)`
	getTradingByPairs, err := db.Preparex(getTradingByQuery)
	if err != nil {
		return nil, nil, err
	}
	return tradingBy, getTradingByPairs, nil
}