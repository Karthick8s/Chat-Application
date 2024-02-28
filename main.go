package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

type User struct {
	ID       int    `json:"id"`
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

type Message struct {
	ID         int       `json:"id"`
	SenderID   int       `json:"sender_id"`
	ReceiverID int       `json:"receiver_id"`
	GroupID    *int      `json:"group_id,omitempty"`
	Content    string    `json:"content"`
	SentAt     time.Time `json:"sent_at"`
}

type Group struct {
	ID        int       `json:"id"`
	GroupName string    `json:"group_name"`
	CreatorID int       `json:"creator_id"`
	CreatedAt time.Time `json:"created_at"`
}

type GroupMembers struct {
	ID      int `json:"id"`
	UserID  int `json:"user_id"`
	GroupID int `json:"group_id"`
}

var db *sql.DB

func main() {

	// DB Connection
	var err error
	conn := "root:openbuddy@tcp(localhost:3306)/chatapp"
	db, err = sql.Open("mysql", conn)
	if err != nil {
		log.Fatal("Error opening database ", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(err.Error())
	}

	router := gin.Default()

	// User Login
	router.POST("chatapp/signup", userSignUp)
	router.POST("chatapp/login", userLogin)

	// Individual Message
	router.POST("chatapp/sendmsg", SendMessage)
	router.GET("chatapp/:id/getmsgs", GetMessages)

	// Group Chat
	router.POST("chatapp/creategroup", CreateGroup)
	router.POST("chatapp/addusertogroup", AddUserToGroup)

	router.Run(":0808")

}

func userSignUp(c *gin.Context) {
	fmt.Println("USER SIGNUP")

	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Insert user into database
	registerQuery := "INSERT INTO users (user_name , password) VALUES (? , ?)"
	_, err := db.Exec(registerQuery, user.UserName, user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		fmt.Println(err)
		return
	}

	c.JSON(http.StatusCreated, "Successfully Registered")
}

func userLogin(c *gin.Context) {
	fmt.Println("USER LOGIN")

	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var storedPassword string

	loginQuery := "SELECT PASSWORD FROM USERS WHERE user_name = ? "
	err := db.QueryRow(loginQuery, user.UserName).Scan(&storedPassword)
	switch {
	case err == sql.ErrNoRows:
		c.JSON(http.StatusUnauthorized, "Invalid username")
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, "Failed to login")
		fmt.Println(err)
		return
	}

	if storedPassword != user.Password {
		c.JSON(http.StatusUnauthorized, "Invalid Password")
		return
	}

	c.JSON(http.StatusAccepted, "Login Success !")
}

func SendMessage(c *gin.Context) {
	var message Message

	if err := c.ShouldBindJSON(&message); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message.SentAt = time.Now()
	fmt.Println(message.SentAt)

	//Check if message is for a group
	if message.GroupID != nil {
		groupMsg := "INSERT INTO groups (group_id , sender_id , content) VALUES (?,?,?)"
		_, err := db.Exec(groupMsg, message.GroupID, message.SenderID, message.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, "Failed to send message to groups")
			return
		}
	} else {
		individualMsg := "INSERT INTO messages (sender_id , receiver_id  , content) VALUES (?,?,?)"
		_, err := db.Exec(individualMsg, message.SenderID, message.ReceiverID, message.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, "Failed to send message")
			fmt.Println(err)
			return
		}
	}

	c.JSON(http.StatusCreated, "Message Sent")
}

// Need to change
func GetMessages(c *gin.Context) {
	userID := c.Param("id")

	// Query messages sent to the user directly
	directMessages, err := getDirectMessages(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Query group messages the user part of
	groupMessages, err := getGroupMessages(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	messages := append(directMessages, groupMessages...)
	fmt.Println("Messages :\n", messages)
	c.JSON(http.StatusOK, messages)
}

func getDirectMessages(userID string) ([]Message, error) {
	query := "SELECT id , sender_id , receiver_id , group_id , content , sent_at from messages where sender_id = ?  and group_id is null"
	fmt.Println(query)
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return ScanMessages(rows)
}

func getGroupMessages(userID string) ([]Message, error) {
	query := "SELECT m.id, m.sender_id , m.receiver_id , m.group_id , m.content , m.sent_at FROM messages m JOIN group_members gm ON m.group_id = gm.group_id  where gm.user_id = ?"

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return ScanMessages(rows)
}

func ScanMessages(rows *sql.Rows) ([]Message, error) {
	var messages []Message
	var err error
	for rows.Next() {
		var message Message
		var sentAt string
		if err = rows.Scan(&message.ID, &message.SenderID, &message.ReceiverID, &message.GroupID, &message.Content, &sentAt); err != nil {
			return nil, err
		}
		message.SentAt, err = time.Parse("2006-01-02 15:04:05", sentAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}

// Create Group to Chat
func CreateGroup(c *gin.Context) {
	var group Group

	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	group.CreatedAt = time.Now()
	createGroupQuery := "INSERT INTO groups (group_name , creator_id) VALUES (? , ?)"
	result, err := db.Exec(createGroupQuery, group.GroupName, group.CreatorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get the groupID from newly created group

	groupID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get group ID"})
		return
	}

	err = AddMembersToTheGroup(group.CreatorID, int(groupID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, "Group Created")
}

func AddUserToGroup(c *gin.Context) {

	var groupMember GroupMembers
	if err := c.ShouldBindJSON(&groupMember); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if the user exists
	userID := groupMember.UserID
	userExists, err := CheckUserExists(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Failed To check user existence")
		return
	}
	if !userExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User Not Found"})
		return
	}

	//Check if the group exists
	groupID := groupMember.GroupID
	groupExists, err := CheckGroupExists(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Failed To check group existence")
		return
	}

	if !groupExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group Not Found"})
		return
	}

	err = AddMembersToTheGroup(userID, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Failed To add users in the group")
		return
	}

	c.JSON(http.StatusCreated, "User added to the group")
}

// Common function to add user in a group
func AddMembersToTheGroup(userID int, groupID int) error {
	addUserToGroupQuery := "INSERT INTO group_members (user_id , group_id) VALUES (? , ?)"
	_, err := db.Exec(addUserToGroupQuery, userID, groupID)
	return err
}

func CheckUserExists(userID int) (bool, error) {

	userExists := "SELECT EXISTS(SELECT 1 FROM 	users where id = ?)"
	var exists bool
	err := db.QueryRow(userExists, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func CheckGroupExists(groupID int) (bool, error) {
	groupExists := "SELECT EXISTS(SELECT 1 FROM groups where id = ?)"
	var exists bool
	err := db.QueryRow(groupExists, groupID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil

}
