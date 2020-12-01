package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode"

	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type authData struct {
	Access_Token string
	Instance_URL string
	ID           string
	Token_Type   string
	Issued_At    string
	Signature    string
}

type attributes struct {
	Type string
	URL  string
}

type recordsCheck struct {
	Attributes attributes
	Email      string
	Name       string
}

type check struct {
	TotalSize int
	Done      bool
	Records   []recordsCheck
}

type infoUsers struct {
	URL  string
	name string
}

type bodyReq struct {
	Email string
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
	//rds := os.Getenv("RDS")

	auth := getAccessToken(clientID, clientSecret, username, password, securityToken)
	users := getUsersWithoutEnterpriseEmail(auth)

	// Sort simple name (Prénom Nom) and change the user's email on SF
	// For more complex name do it by yourself..
	for _, st := range users {
		name := strings.Split(st.name, " ")
		if len(name) == 2 {
			processEmailChange(auth, st.URL, name)
		}
	}
}

func processEmailChange(auth authData, Uurl string, name []string) {
	completeName := strings.ToLower(name[0] + "." + name[1])
	// Set the URL and the Bearer for the http request
	url := "https://eu9.salesforce.com/" + Uurl
	bearer := "Bearer " + auth.Access_Token

	newMail := verifAccent(completeName + "@enterprise.io")

	// Create the body with a type struct
	var jsonStr, _ = json.Marshal(bodyReq{
		Email: newMail})

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
	if string(body) != "" && resp.StatusCode != 204 {
		log.Println(name, string(body), resp.Status)
	}
}

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

func verifAccent(s string) string {
	b := make([]byte, len(s))

	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	_, _, e := t.Transform(b, []byte(s), true)
	if e != nil {
		panic(e)
	}

	return string(b)
}

func getUsersWithoutEnterpriseEmail(auth authData) map[string]infoUsers {
	without := make(map[string]infoUsers)
	url := "https://eu9.salesforce.com/services/data/v40.0/query/?q=SELECT+Email,Name+from+Contact+Where+Type_de_contact__c='Salarié'+AND+A_quitte_la_societe__c=False"

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

	for _, r := range checkIt.Records {
		if !(strings.Contains(r.Email, "enterprise.io")) {
			without[r.Email] = infoUsers{URL: r.Attributes.URL, name: r.Name}
		}
	}

	return without
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
