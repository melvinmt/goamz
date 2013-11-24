package dynamodb

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alimoeeny/goamz/aws"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	Auth            aws.Auth
	Region          aws.Region
	CapacityChannel chan map[string]float64
}

/*
type Query struct {
	Query string
}
*/

/*
func NewQuery(queryParts []string) *Query {
	return &Query{
		"{" + strings.Join(queryParts, ",") + "}",
	}
}
*/

// ALI
// func (s *Server) QueryServer(target string, query *Query) ([]byte, error) {
// 	return s.queryServer(target, query)
// }
// Specific error constants
var ErrNotFound = errors.New("Item not found")

// Error represents an error in an operation with Dynamodb (following goamz/s3)
type Error struct {
	StatusCode int // HTTP status code (200, 403, ...)
	Status     string
	Code       string // Dynamodb error code ("MalformedQueryString", ...)
	Message    string // The human-oriented error message
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

func buildError(r *http.Response, jsonBody []byte) error {

	ddbError := Error{
		StatusCode: r.StatusCode,
		Status:     r.Status,
	}
	// TODO return error if Unmarshal fails?

	var js map[string]string
	err := json.Unmarshal(jsonBody, &js)
	if err != nil {
		log.Printf("Failed to parse body as JSON")
		return err
	}
	ddbError.Message = js["message"]

	// Of the form: com.amazon.coral.validate#ValidationException
	// We only want the last part
	codeStr := js["__type"]
	hashIndex := strings.Index(codeStr, "#")
	if hashIndex > 0 {
		codeStr = codeStr[hashIndex+1:]
	}
	ddbError.Code = codeStr

	return &ddbError
}

func (s *Server) queryServer(target string, query *Query) ([]byte, error) {
	data := strings.NewReader(query.String())

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// 	// 	s := "{
	// 	//     \"TableName\": \"CSUsersEmail\",
	// 	//     \"IndexName\": \"LastPostIndex\",
	// 	//     \"Select\": \"ALL_ATTRIBUTES\",
	// 	//     \"Limit\":3,
	// 	//     \"ConsistentRead\": true,
	// 	//     \"KeyConditions\": {
	// 	//         \"LastPostDateTime\": {
	// 	//             \"AttributeValueList\": [
	// 	//                 {
	// 	//                     \"S\": \"20130101\"
	// 	//                 },
	// 	//                 {
	// 	//                     \"S\": \"20130115\"
	// 	//                 }
	// 	//             ],
	// 	//             \"ComparisonOperator\": \"BETWEEN\"
	// 	//         },
	// 	//         \"ForumName\": {
	// 	//             \"AttributeValueList\": [
	// 	//                 {
	// 	//                     \"S\": \"Amazon DynamoDB\"
	// 	//                 }
	// 	//             ],
	// 	//             \"ComparisonOperator\": \"EQ\"
	// 	//         }
	// 	//     },
	// 	//     \"ReturnConsumedCapacity\": \"TOTAL\"
	// 	// }"

	// 	sdata := `{
	//     "TableName": "CSUsersEmail",
	//     "Select": "ALL_ATTRIBUTES",
	//     "Limit": 3,
	//     "ConsistentRead": true,
	//     "KeyConditions": {
	//         "PK_EMAIL": {
	//             "AttributeValueList": [
	//                 {
	//                     "S": "a"
	//                 },
	//                 {
	//                     "S": "z"
	//                 }
	//             ],
	//             "ComparisonOperator": "BETWEEN"
	//         }
	//     },
	//     "ReturnConsumedCapacity": "TOTAL"
	// }`

	// 	data = strings.NewReader(sdata)

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	hreq, err := http.NewRequest("POST", s.Region.DynamoDBEndpoint+"/", data)
	if err != nil {
		return nil, err
	}

	hreq.Header.Set("Content-Type", "application/x-amz-json-1.0")

	//ALI
	if s.Auth.Token() != "" {
		hreq.Header.Set("X-Amz-Security-Token", s.Auth.Token())
	}

	hreq.Header.Set("X-Amz-Date", time.Now().UTC().Format(aws.ISO8601BasicFormat))
	hreq.Header.Set("X-Amz-Target", target)

	signer := aws.NewV4Signer(s.Auth, "dynamodb", s.Region)
	signer.Sign(hreq)

	resp, err := http.DefaultClient.Do(hreq)

	if err != nil {
		fmt.Printf("Error calling Amazon:%v\n", err)
		fmt.Println("hreq is:", hreq)
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("Could not read response body")
		return nil, err
	}

	// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/ErrorHandling.html
	// "A response code of 200 indicates the operation was successful."
	if resp.StatusCode != 200 {
		ddbErr := buildError(resp, body)
		return nil, ddbErr
	}

	return body, nil
}

func target(name string) string {
	return "DynamoDB_20111205." + name
}
