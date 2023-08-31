package shopwaredb

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/RoanBrand/SpectroDashboard/config"
	"github.com/RoanBrand/SpectroDashboard/log"
	"github.com/RoanBrand/SpectroDashboard/sample"
	"github.com/RoanBrand/SpectroDashboard/xml_spectro/fileparser"
	_ "github.com/denisenkom/go-mssqldb"
)

type ShopwareDB struct {
	conf       *config.Config
	db         *sql.DB
	connString string
}

func SetupShopwareDB(conf *config.Config) *ShopwareDB {
	c := &conf.ShopwareDB

	sdb := &ShopwareDB{
		conf:       conf,
		connString: fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s", c.Address, c.User, c.Password, c.Database),
	}

	err := sdb.openDB()
	if err != nil {
		log.Println(err)
	}

	return sdb
}

func (sdb *ShopwareDB) Stop() error {
	if sdb.db == nil {
		return nil
	}

	if err := sdb.db.Close(); err != nil {
		return fmt.Errorf("failed closing shopware DB: %w", err)
	}

	return nil
}

func (sdb *ShopwareDB) openDB() error {
	db, err := sql.Open("mssql", sdb.connString)
	if err != nil {
		return fmt.Errorf("failed opening shopware DB: %w", err)
	}

	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed pinging after opening shopware DB: %w", err)
	}

	sdb.db = db
	return nil
}

// Insert new results from spectro machines into foundry's Shopware MS SQL Server database.
func (sdb *ShopwareDB) InsertNewResults(samples []*sample.Record, debug bool) error {
	if sdb.db == nil {
		err := sdb.openDB()
		if err != nil {
			return err
		}
	}

	tx, err := sdb.db.Begin()
	if err != nil {
		return err
	}

	var lastTime time.Time
	if err = tx.QueryRow(`SELECT TOP (1) DateTimeStamp FROM ` + sdb.conf.ShopwareDB.Table + ` ORDER BY ID DESC;`).Scan(&lastTime); err != nil {
		if err != sql.ErrNoRows {
			tx.Rollback()
			return err
		}
	}

	// We insert wall time (without TZ), so DB returns as UTC. Convert here to SAST, preserving wall clock time.
	lastTime, err = time.ParseInLocation("2006-01-02 15:04:05", lastTime.Format("2006-01-02 15:04:05"), time.Local)
	if err != nil {
		return err
	}

	if debug {
		log.Printf("remote DB last sample timestamp: %s\n", lastTime)
	}

	extraElementsToAdd := []string{"Ni", "Mo", "Co", "Nb", "V", "W", "Mg", "Bi", "Ca", "As", "Sb", "Te", "Fe"}

	for i := len(samples) - 1; i >= 0; i-- {
		s := samples[i]
		if !s.TimeStamp.After(lastTime) {
			continue
		}

		qry := strings.Builder{}
		qry.WriteString(`INSERT INTO "`)
		qry.WriteString(sdb.conf.ShopwareDB.Table)
		qry.WriteString(`" ("DateTimeStamp", "SampleName", "Furname", "Spectro"`)
		for _, r := range s.Results {
			if r.Element == "" {
				continue
			}

			qry.WriteString(`, "`)
			qry.WriteString(r.Element)
			qry.WriteByte('"')
		}
		for _, el := range extraElementsToAdd {
			if _, ok := s.ResultsMap[el]; ok {
				qry.WriteString(`, "`)
				qry.WriteString(el)
				qry.WriteByte('"')
			}
		}
		qry.WriteString(`) VALUES ('`)
		// DB column is DATETIME, with no timezone
		qry.WriteString(s.TimeStamp.Format("2006-01-02 15:04:05"))
		qry.WriteString("', '")
		qry.WriteString(s.SampleName)
		qry.WriteString("', '")
		qry.WriteString(s.Furnace)
		qry.WriteString("', ")
		qry.WriteString(strconv.Itoa(s.Spectro))

		for _, r := range s.Results {
			if r.Element == "" {
				continue
			}

			qry.WriteString(`, `)
			qry.WriteString(strconv.FormatFloat(r.Value, 'f', 8, 64))
		}
		for _, el := range extraElementsToAdd {
			if elRes, ok := s.ResultsMap[el]; ok {
				qry.WriteString(`, `)
				qry.WriteString(strconv.FormatFloat(elRes, 'f', 8, 64))
			}
		}

		qry.WriteString(");")

		q := qry.String()
		if debug {
			log.Println("remote DB query: " + q)
		}
		if _, err := tx.Exec(q); err != nil {
			tx.Rollback()
			return errors.New("error executing insert statement: " + q + " Error: " + err.Error())
		}
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func (sdb *ShopwareDB) InsertNewXMLResults(recs []fileparser.Record) error {
	if sdb.db == nil {
		err := sdb.openDB()
		if err != nil {
			return err
		}
	}

	tx, err := sdb.db.Begin()
	if err != nil {
		return err
	}

	qry := strings.Builder{}
	qry.WriteString(`SELECT TOP (1) DateTimeStamp FROM "`)
	qry.WriteString(sdb.conf.ShopwareDB.Table)
	qry.WriteString(`" WHERE "Spectro" = @p1`)
	qry.WriteString(` ORDER BY ID DESC;`)

	var lastTime time.Time
	if err = tx.QueryRow(qry.String(), sdb.conf.SpectroNumber).Scan(&lastTime); err != nil {
		if err != sql.ErrNoRows {
			tx.Rollback()
			return err
		}
	}

	// We insert wall time (without TZ), so DB returns as UTC. Convert here to SAST, preserving wall clock time.
	lastTime, err = time.ParseInLocation("2006-01-02 15:04:05", lastTime.Format("2006-01-02 15:04:05"), time.Local)
	if err != nil {
		return err
	}

	if sdb.conf.DebugMode {
		log.Printf("remote DB last sample timestamp: %s\n", lastTime)
	}

	elementsToInsert := []string{"C", "Si", "Mn", "P", "S", "Cu", "Cr", "Al", "Ti", "Sn", "Zn", "Pb",
		"Ni", "Mo", "Co", "Nb", "V", "W", "Mg", "Bi", "Ca", "As", "Sb", "Te", "Fe"}

	for i := len(recs) - 1; i >= 0; i-- { // reverse order: older to newer
		s := recs[i]
		if !s.TimeStamp.After(lastTime) {
			continue
		}

		qry.Reset()
		qry.WriteString(`INSERT INTO "`)
		qry.WriteString(sdb.conf.ShopwareDB.Table)
		qry.WriteString(`" ("DateTimeStamp", "SampleName", "Furname", "Spectro"`)

		for _, el := range elementsToInsert {
			if _, ok := s.Results[el]; ok {
				qry.WriteString(`, "`)
				qry.WriteString(el)
				qry.WriteByte('"')
			}
		}
		qry.WriteString(`) VALUES ('`)
		// DB column is DATETIME, with no timezone
		qry.WriteString(s.TimeStamp.Format("2006-01-02 15:04:05"))
		qry.WriteString("', '")
		qry.WriteString(s.ID)
		qry.WriteString("', '")
		qry.WriteString(s.Furnace)
		qry.WriteString("', ")
		qry.WriteString(strconv.Itoa(sdb.conf.SpectroNumber))

		for _, el := range elementsToInsert {
			if elRes, ok := s.Results[el]; ok {
				qry.WriteString(`, `)
				qry.WriteString(strconv.FormatFloat(elRes, 'f', 8, 64))
			}
		}

		qry.WriteString(");")

		q := qry.String()
		if sdb.conf.DebugMode {
			log.Println("remote DB query: " + q)
		}
		if _, err := tx.Exec(q); err != nil {
			tx.Rollback()
			return errors.New("error executing insert statement: " + q + " Error: " + err.Error())
		}
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return err
	}

	return nil
}
