package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"test_bot/m"
)

var (
	queryCheck    = `SELECT * FROM prods WHERE id = ?`
	queryAddList  = `INSERT INTO tokens (list) VALUES (?)`
	queryFindList = `SELECT * FROM tokens WHERE id = ?`
	queryUpdate   = `UPDATE tokens SET list = ? WHERE id = ?`
)

type Prod struct {
	ID   int
	Name string
	Qty  int
}

type DB struct {
	*sql.DB
}

// CheckQty return quantity of products (only one product at a time)
func (db *DB) CheckQty(msg string) (string, error) {

	msgList := strings.Fields(msg)
	cmd := strings.TrimSpace(msgList[0])

	if len(msgList) != 2 {
		return "", errors.New(m.WrongFormat)
	}
	if cmd != "п" {
		return "", errors.New(m.WrongFormat)
	}

	//numberQty[0] - is command. Must be ignored in this method.
	id, err := strconv.Atoi(msgList[1])
	if err != nil {
		return "", fmt.Errorf("ошибка в коде товара: %w", err)
	}

	row := db.QueryRow(queryCheck, id)

	var id2, qty2, price2 int
	var name2 string

	err = row.Scan(&id2, &name2, &qty2, &price2)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("не найден код товара \"%d\": %w", id, err)
	}
	if err != nil {
		return "", fmt.Errorf("системный сбой: %w", err)
	}

	return fmt.Sprintf("%d %s - %d", id2, name2, qty2), nil
}

// CreateNewProdlist create new list of products and return his ID
func (db *DB) CreateNewProdlist(msg string) (string, error) {

	//ignore command msgList[0]
	msgList := strings.Split(msg, "\n")
	truncatedMsg := strings.Join(msgList[1:], "\n")

	//catching errors
	cmd := strings.TrimSpace(msgList[0])

	switch {
	case len(msgList) < 2:
		return "", errors.New(m.WrongFormat)

	case cmd != "с":
		return "", errors.New(m.WrongFormat)
	}

	//create list of products
	prodlist, err := parseProdlist(truncatedMsg)
	if err != nil {
		return "", err
	}

	//checking for correct input
	for _, p := range prodlist {
		err := db.check(p.ID, p.Qty)
		if err != nil {
			return "", err
		}
	}

	//convert list to JSON
	data, err := convToJSON(prodlist)
	if err != nil {
		return "", err
	}

	//insert to database
	res, err := db.Exec(queryAddList, data)
	if err != nil {
		return "", err
	}

	//get a number of list
	listID, err := res.LastInsertId()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Номер вашего жетона: %d", listID), err
}

// AddToProdlist add new list from msg to old list from database
func (db *DB) AddToProdlist(msg string) (string, error) {

	msgList := strings.Split(msg, "\n")

	cmd := strings.TrimSpace(msgList[0])

	//catching errors
	switch {
	case len(msgList) < 3:
		return "", errors.New(m.WrongFormat)
	case cmd != "д":
		return "", errors.New(m.WrongFormat)
	}

	//convert and catching error in number of list
	number := strings.TrimSpace(msgList[1])
	listID, err := strconv.Atoi(number)
	if err != nil {
		return "", fmt.Errorf("ошибка в номере жетона: %w", err)
	}

	//cut off command and listID
	truncatedMsg := strings.Join(msgList[2:], "\n")

	//new list of product
	newListprod, err := parseProdlist(truncatedMsg)
	if err != nil {
		return "", err
	}

	//check new list in db
	for _, v := range newListprod {
		err := db.check(v.ID, v.Qty)
		if err != nil {
			return "", err
		}
	}

	//get old list
	oldListprod, err := db.getListFromDB(listID)
	if err != nil {
		return "", err
	}

	//adding new list to old list
	oldListprod = append(oldListprod, newListprod...)

	//convert to json
	data, err := convToJSON(oldListprod)
	if err != nil {
		return "", err
	}

	//update list
	_, err = db.Exec(queryUpdate, data, listID)
	if err != nil {
		return "", err
	}

	//convert to string and return
	return prodListToString(oldListprod), err
}

// RemoveFromProdlist remove only one product at a time
func (db *DB) RemoveFromProdlist(msg string) (string, error) {

	msgList := strings.Split(msg, "\n")

	cmd := strings.TrimSpace(msgList[0])

	//catching errors
	switch {
	case len(msgList) != 3:
		return "", errors.New(m.WrongFormat)
	case cmd != "у":
		return "", errors.New(m.WrongFormat)
	}

	//convert and catching error in number of list
	number := strings.TrimSpace(msgList[1])
	listID, err := strconv.Atoi(number)
	if err != nil {
		return "", fmt.Errorf("ошибка в номере жетона: %w", err)
	}

	//get an id of product
	badID := strings.TrimSpace(msgList[2])
	id, err := strconv.Atoi(badID)
	if err != nil {
		return "", fmt.Errorf("ошибка в коде товара: %w", err)
	}

	//get list from database
	listprod, err := db.getListFromDB(listID)
	if err != nil {
		return "", err
	}

	//remove first match
	//if not - return error
	isHave := false
	for i, p := range listprod {
		if p.ID == id {
			isHave = true
			listprod = append(listprod[:i], listprod[i+1:]...)
			break
		}
	}
	if !isHave {
		return "", errors.New(m.NoMatch)
	}

	//convert to JSON
	data, err := convToJSON(listprod)
	if err != nil {
		return "", err
	}

	//update list
	_, err = db.Exec(queryUpdate, data, listID)
	if err != nil {
		return "", err
	}

	//convert to string and return
	return prodListToString(listprod), err
}

// ShowProdlist get list of product by id in string format
func (db *DB) ShowProdlist(msg string) (string, error) {

	msgList := strings.Fields(msg)

	//ignore command msgList[0] and get id of list
	listID, err := strconv.Atoi(msgList[1])
	if err != nil {
		return "", fmt.Errorf("ошибка в номере жетона: %w", err)
	}

	//get list from database
	listprod, err := db.getListFromDB(listID)
	if err != nil {
		return "", err
	}

	//convert list to string and return
	return prodListToString(listprod), err
}

// convToJSON convert []Prod to JSON
func convToJSON(list []Prod) ([]byte, error) {
	data, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// convFromJson convert JSON to []Prod
func convFromJson(data []byte) ([]Prod, error) {
	listProd := []Prod{}
	err := json.Unmarshal(data, &listProd)
	if err != nil {
		return nil, err
	}
	return listProd, nil
}

// check return err if ID not exist and not enough Qty.
func (db *DB) check(id, qty int) error {

	row := db.QueryRow(queryCheck, id)

	var id2, qty2, price2 int
	var name2 string
	err := row.Scan(&id2, &name2, &qty2, &price2)

	if err == sql.ErrNoRows {
		return fmt.Errorf("не найден код товара \"%d\": %w", id, err)
	} else if err != nil {
		return fmt.Errorf("ошибка в базе данных: %w", err)
	} else if qty2-qty < 0 {
		return fmt.Errorf("недостаточное кол-во, всего в наличии: %d", qty2)
	}
	return nil
}

// getListFromDB get JSON from database and convert to []Prod
func (db *DB) getListFromDB(listID int) ([]Prod, error) {

	var data []byte
	var id int

	row := db.QueryRow(queryFindList, listID)

	err := row.Scan(&id, &data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("жетон не найден: %w", err)
	}
	if err != nil {
		return nil, err
	}

	listProd, err := convFromJson(data)
	if err != nil {
		return nil, err
	}

	return listProd, nil
}

// parseProdlist convert incoming message to []Prod
func parseProdlist(truncatedMsg string) ([]Prod, error) {

	var prodlist []Prod

	list := strings.Split(truncatedMsg, "\n")

	for _, p := range list {

		pr := strings.Fields(p)

		//catch incorrect input prod
		if len(pr) != 2 {
			return nil, errors.New(m.WrongFormat)
		}

		i, err := strconv.Atoi(pr[0])
		if err != nil {
			return nil, fmt.Errorf("ошибка в коде товара: %w", err)
		}

		q, err := strconv.Atoi(pr[1])
		if err != nil {
			return nil, fmt.Errorf("ошибка в указании кол-ва: %w", err)
		}

		prodlist = append(prodlist, Prod{ID: i, Qty: q})

	}
	return prodlist, nil
}

func (p Prod) String() string {

	return fmt.Sprintf("%d %s -  %d", p.ID, p.Name, p.Qty)
}

func prodListToString(pl []Prod) string {
	listString := make([]string, len(pl))
	for i, v := range pl {
		listString[i] = v.String()
	}

	return strings.Join(listString, "\n")
}
