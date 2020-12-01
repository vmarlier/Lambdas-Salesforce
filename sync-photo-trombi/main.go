package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

type authData struct {
	Access_Token string
	Instance_URL string
	ID           string
	Token_Type   string
	Issued_At    string
	Signature    string
}

type attributesEmail struct {
	Type string
	URL  string
}

type recordsEmail struct {
	Attributes attributesEmail
	Email      string
	Name       string
}

type usersEmail struct {
	TotalSize      int
	Done           bool
	NextRecordsURL string
	Records        []recordsEmail
}

type attributesCheck struct {
	Type string
	url  string
}

type recordsCheck struct {
	Attributes attributesCheck
	Photos2__c string
}

type check struct {
	TotalSize int
	Done      bool
	Records   []recordsCheck
}

type bodyReq struct {
	Photos2__c string
}

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(ctx context.Context) {
	// Get credentials from lambda env
	clientID := os.Getenv("CLIENTID")
	clientSecret := os.Getenv("CLIENTSECRET")
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	securityToken := os.Getenv("SECURITYTOKEN")
	rds := os.Getenv("RDS")

	auth := getAccessToken(clientID, clientSecret, username, password, securityToken)
	fmt.Println(auth)
	mail := getUsersEmail(auth)
	fmt.Println(mail)

	for _, r := range mail {
		if asSalesforcePhoto(auth, r.Email) == false {
			query := sqlQuery(formatEmail(r.Email), rds)
			if query != "No rows were returned!" {
				resp := setPhotoSalesforce(auth, r.Attributes.URL, query, r.Name)
				if resp == true {
					log.Printf("%s has seen his photo updated \n", r.Name)
				}
			}
		}
	}
}

// Set the photo in salesforce for a given user
func setPhotoSalesforce(auth authData, Uurl string, path string, name string) bool {
	img := "<img src=\"" + path + "\" alt=\"" + name + "\" style=\"height:100px; width:100px;\" border=\"0\"/>"

	// Set the URL and the Bearer for the http request
	url := "https://eu9.salesforce.com/" + Uurl
	bearer := "Bearer " + auth.Access_Token

	// Create the body with a type struct
	var jsonStr, _ = json.Marshal(bodyReq{
		Photos2__c: img})

	// Create a new request using http
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

	// Add authorization header to the req
	req.Header.Add("Authorization", bearer)
	req.Header.Set("Content-Type", "application/json")

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	// Close the request at the end (defer)
	defer resp.Body.Close()

	// Get body response
	body, _ := ioutil.ReadAll(resp.Body)

	// If body is empty and status code is 204 validate the patch request
	if string(body) == "" && resp.StatusCode == 204 {
		return true
	} else {
		log.Println(name, string(body), resp.Status)
	}

	return false
}

// Ask database (RDS) to know the actual path of the user's picture
func sqlQuery(email string, rds string) string {
	var path string

	// Define the pattern for the WHERE clause
	like := "'https://enterprise.io/wp-content/uploads/20%/%/" + email + "%'"

	// Define the query
	query := "SELECT guid FROM badges_posts WHERE guid LIKE " + like

	// Initialize the DB connection
	db, err := sql.Open("mysql", rds)
	if err != nil {
		log.Fatal(err)
	}
	// Close the DB with a defer (defer is executed at the end)
	defer db.Close()

	// Get the query result
	row := db.QueryRow(query)
	// Handle query situation
	switch err := row.Scan(&path); err {
	case sql.ErrNoRows:
		return "No rows were returned!"
	case nil:
		return path
	default:
		panic(err)
	}
}

// Transform Email in formatted version to search in DB
// Exemple: "firstname.lastname@enterprise.io" -> "firstname_lastname@enterprise_io"
func formatEmail(email string) (mail string) {
	// Split email in letters
	letter := string([]rune(email))

	// If letter is a point turn it into an underscore
	for _, l := range letter {
		if string(l) == "." {
			mail += "_"
		} else {
			mail += string(l)
		}
	}

	return mail
}

// Check if employee photo is set return a boolean
func asSalesforcePhoto(auth authData, mail string) bool {
	// SOQL https://workbench.developerforce.com/query.php -> useful to know existing fields
	url := "https://eu9.salesforce.com/services/data/v40.0/query/?q=SELECT+Photos2__c+from+Contact+Where+Email='" + mail + "'"

	// Create Bearer
	bearer := "Bearer " + auth.Access_Token

	// Create a new request using http
	req, err := http.NewRequest("GET", url, nil)

	// Add authorization header to the req
	req.Header.Add("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json")

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error on response.\n[ERROR] -", err)
	}

	// Get response body
	body, _ := ioutil.ReadAll(resp.Body)

	// Parse json in a struct
	var checkIt check
	json.Unmarshal(body, &checkIt)

	// Uncomment the lign bellow if an error is detecte but not logged
	// log.Println(checkIt.Records)
	// If returned field Photos2__c is empty so users don't have a profile picture
	for _, r := range checkIt.Records {
		if r.Photos2__c == "" {
			return false
		}
	}
	log.Printf("User: %s, already has a profile picture\n", strings.Split(mail, "@")[0])
	return true
}

// Get all employee email and url
func getUsersEmail(auth authData) []recordsEmail {
	// SOQL both of url bellow works
	url := "https://eu9.salesforce.com/services/data/v40.0/query/?q=SELECT+Email,Name+from+Contact+Where+Type_de_contact__c='Salari%C3%89'+AND+A_quitte_la_societe__c=False+AND+Etapes_r_alis_es__c='Recrute'"
	//url := "https://eu9.salesforce.com/services/data/v40.0/query/?q=SELECT+Email,Name+from+Contact+Where+Etapes_r_alis_es__c='Recrute'+AND+A_quitte_la_societe__c=False+AND+Type_de_contrat__c='CDI'"

	// Create Bearer
	bearer := "Bearer " + auth.Access_Token

	// Create a new request using http
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error on http.NewRequest")
	}

	// Add authorization header to the req
	req.Header.Add("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json")

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error on response.\n[ERROR] -", err)
	}

	// Get the response body
	body, _ := ioutil.ReadAll(resp.Body)

	// Parse the json to a struct
	var mail usersEmail
	json.Unmarshal(body, &mail)

	return mail.Records
}

// Get Authentication token from salesforce
func getAccessToken(clientID, clientSecret, username, password, securityToken string) (result authData) {
	var body []byte

	// Initialize post request
	response, err := http.PostForm("https://login.salesforce.com/services/oauth2/token", url.Values{"grant_type": {"password"}, "client_id": {clientID}, "client_secret": {clientSecret}, "username": {username}, "password": {password + securityToken}})
	if err != nil {
		log.Printf("The HTTP request failed with error %s\n", err)
	} else {
		body, _ = ioutil.ReadAll(response.Body)
	}

	// Parse the json to a struct
	if strings.Contains(string(body), "error") {
		log.Fatal("[ERROR] -- getAccessToken() : ", string(body))
	} else {
		json.Unmarshal(body, &result)
	}

	return result
}
