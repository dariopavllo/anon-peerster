package main

import (
	"database/sql"
	"encoding/hex"
	_ "github.com/mattn/go-sqlite3"
)

type DbConnection struct {
	Connection *sql.DB
}

type MessageRecord struct {
	Data            RumorMessage
	DateSeen        string
	FromAddress     string
	ComputedHashStr string // This field is used just for the GUI
}

func (m *MessageRecord) ComputeHashStr() string {
	binHash := m.Data.ComputeHash()
	m.ComputedHashStr = hex.EncodeToString(binHash)
	return m.ComputedHashStr
}

func NewConnection(dbPath string) *DbConnection {
	db, err := sql.Open("sqlite3", dbPath+"/messages.db")
	FailOnError(err)
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS messages (" +
		"ID INTEGER NOT NULL," +
		"Origin TEXT NOT NULL," +
		"Destination TEXT NOT NULL," +
		"Content BLOB NOT NULL," +
		"Signature BLOB NOT NULL," +
		"Nonce BLOB NOT NULL," +
		"DateSeen TEXT NOT NULL," +
		"FromAddress TEXT NOT NULL," +
		"PRIMARY KEY (ID, Origin)" +
		")")
	FailOnError(err)
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_origin ON messages(Origin)")
	FailOnError(err)
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_dest ON messages(Destination)")
	FailOnError(err)
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_origin_dest ON messages(Origin, Destination)")
	FailOnError(err)
	return &DbConnection{db}
}

func (db *DbConnection) NextID(nodeName string) uint32 {
	stmt, err := db.Connection.Prepare("SELECT MAX(ID) FROM messages WHERE Origin = ?")
	FailOnError(err)
	defer stmt.Close()
	result, err := stmt.Query(nodeName)
	FailOnError(err)
	defer result.Close()

	var id uint32
	if result.Next() {
		var id_ sql.NullInt64
		result.Scan(&id_)
		if id_.Valid {
			id = uint32(id_.Int64) + 1
		} else {
			id = 0
		}
	} else {
		id = 0
	}
	return id
}

func (db *DbConnection) VectorClock() []PeerStatus {
	result, err := db.Connection.Query("SELECT Origin, MAX(ID) FROM messages GROUP BY Origin")
	FailOnError(err)
	defer result.Close()

	status := make([]PeerStatus, 0)
	for result.Next() {
		var origin string
		var id uint32
		result.Scan(&origin, &id)
		status = append(status, PeerStatus{origin, id + 1})
	}
	return status
}

func (db *DbConnection) NodeList() []string {
	result, err := db.Connection.Query("SELECT DISTINCT Origin FROM messages ORDER BY Origin ASC")
	FailOnError(err)
	defer result.Close()

	nodes := make([]string, 0)
	for result.Next() {
		var origin string
		result.Scan(&origin)
		nodes = append(nodes, origin)
	}
	return nodes
}

func (db *DbConnection) InsertOrUpdateMessage(m *MessageRecord) {
	// Start a new transaction
	tx, err := db.Connection.Begin()
	FailOnError(err)

	// Delete the message if it already exists
	stmt, err := tx.Prepare("DELETE FROM messages WHERE Origin = ? AND ID = ?")
	FailOnError(err)
	_, err = stmt.Exec(m.Data.Origin, m.Data.ID)
	FailOnError(err)
	stmt.Close()

	// Insert the new message
	stmt, err = tx.Prepare("INSERT INTO messages(ID, Origin, Destination, Content, Signature, Nonce, " +
		"DateSeen, FromAddress) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	_, err = stmt.Exec(m.Data.ID, m.Data.Origin, m.Data.Destination, m.Data.Content,
		m.Data.Signature, m.Data.Nonce, m.DateSeen, m.FromAddress)
	FailOnError(err)
	stmt.Close()

	// Commit transaction
	FailOnError(tx.Commit())
}

func (db *DbConnection) GetMessage(origin string, id uint32) *MessageRecord {
	stmt, err := db.Connection.Prepare("SELECT Destination, Content, Signature, Nonce," +
		"DateSeen, FromAddress FROM messages WHERE Origin = ? AND ID = ?")
	FailOnError(err)
	defer stmt.Close()

	result, err := stmt.Query(origin, id)
	FailOnError(err)
	defer result.Close()

	if result.Next() {
		m := &MessageRecord{}
		m.Data.Origin = origin
		m.Data.ID = id
		result.Scan(&m.Data.Destination, &m.Data.Content, &m.Data.Signature,
			&m.Data.Nonce, &m.DateSeen, &m.FromAddress)
		if len(m.Data.Content) == 0 {
			m.Data.Content = make([]byte, 0) // Fix for serialization
		}
		if len(m.Data.Signature) == 0 {
			m.Data.Signature = make([]byte, 0) // Fix for serialization
		}
		return m
	} else {
		return nil
	}
}

func (db *DbConnection) GetAllMessagesTo(destination string) []*MessageRecord {
	stmt, err := db.Connection.Prepare("SELECT ID, Origin, Content, Signature, Nonce," +
		"DateSeen, FromAddress FROM messages WHERE Destination = ?")
	FailOnError(err)
	defer stmt.Close()

	result, err := stmt.Query(destination)
	FailOnError(err)
	defer result.Close()

	output := make([]*MessageRecord, 0)
	for result.Next() {
		m := &MessageRecord{}
		m.Data.Destination = destination
		result.Scan(&m.Data.ID, &m.Data.Origin, &m.Data.Content, &m.Data.Signature,
			&m.Data.Nonce, &m.DateSeen, &m.FromAddress)
		output = append(output, m)
	}

	return output
}

func (db *DbConnection) GetAllMessagesBetween(origin string, destination string) []*MessageRecord {
	stmt, err := db.Connection.Prepare("SELECT ID, Origin, Destination, Content, Signature, Nonce," +
		"DateSeen, FromAddress FROM messages WHERE (Origin = ? AND Destination = ?)" +
		"OR (Origin = ? AND Destination = ?) ORDER BY DateSeen ASC")
	FailOnError(err)
	defer stmt.Close()

	result, err := stmt.Query(origin, destination, destination, origin)
	FailOnError(err)
	defer result.Close()

	output := make([]*MessageRecord, 0)
	for result.Next() {
		m := &MessageRecord{}
		result.Scan(&m.Data.ID, &m.Data.Origin, &m.Data.Destination, &m.Data.Content, &m.Data.Signature,
			&m.Data.Nonce, &m.DateSeen, &m.FromAddress)
		output = append(output, m)
	}

	return output
}
