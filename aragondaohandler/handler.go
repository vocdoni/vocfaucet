package aragondaohandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
)

func VerifyAragonDaoRequest(data string, signature types.HexBytes) (common.Address, error) {
	// Obtain the address from the signature
	addr, err := ethereum.AddrFromSignature([]byte(data), signature)
	if err != nil {
		return addr, err
	}

	type SignatureData struct {
		Message string `json:"message"`
		Date    string `json:"date"`
	}
	signatureData := SignatureData{}
	if err := json.Unmarshal([]byte(data), &signatureData); err != nil {
		return addr, err
	}

	// Check if the signature provided date is from today
	parsedDate, err := time.Parse("2006-01-02", signatureData.Date)
	if err != nil {
		return addr, err
	}
	currentDate := time.Now()
	if currentDate.Year() != parsedDate.Year() ||
		currentDate.Month() != parsedDate.Month() ||
		currentDate.Day() != parsedDate.Day() {
		return addr, errors.New("signature date is not from today")
	}

	return addr, nil
}


func IsAragonDaoAddress(addr common.Address, network string) (bool, error) {
	if network != "mainnet" && network != "goerli" && network != "sepolia" && network != "polygon" && network != "mumbai" {
		return false, errors.New("network not supported")
	}

	query := []byte(`
		query DaoMembers($address: String, $address2: Bytes) {
			multisigPlugins(where: {members_: {address: $address}}) {
				members(where: {address: $address}) {
				address
				}
			}
			tokenVotingPlugins(where: {members_: {address: $address2}}) {
				members(where: {address: $address2}) {
				address
				}
			}
			addresslistVotingPlugins(where: {members_: {address: $address}}) {
				members(where: {address: $address}) {
				address
				}
			}
		}
	`)

	// use the network to determine the subgraphURL
	graphURL := "https://subgraph.satsuma-prod.com/qHR2wGfc5RLi6/aragon/osx-" + string(network) + "/version/v1.3.0/api"

	variables := map[string]interface{}{
		"address":  addr,
		"address2": addr,
	}
	requestData := map[string]interface{}{
		"query":     string(query),
		"variables": variables,
	}

	var err error
	var requestBody []byte
	if requestBody, err = json.Marshal(requestData); err != nil {
		return false, err
	}

	var req *http.Request
	var resp *http.Response
	if req, err = http.NewRequest("POST", graphURL, bytes.NewBuffer(requestBody)); err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	if resp, err = client.Do(req); err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return false, err
	}

	// Parse the JSON response
	type Response struct {
		Data struct {
			MultisigPlugins []struct {
				Members []struct {
					Address string `json:"address"`
				} `json:"members"`
			} `json:"multisigPlugins"`
			TokenVotingPlugins []struct {
				Members []struct {
					Address string `json:"address"`
				} `json:"members"`
			} `json:"tokenVotingPlugins"`
			AddresslistVotingPlugins []struct {
				Members []struct {
					Address string `json:"address"`
				} `json:"members"`
			} `json:"addresslistVotingPlugins"`
		} `json:"data"`
	}

	response := Response{}
	if err := json.Unmarshal(body, &response); err != nil {
		return false, err
	}

	if len(response.Data.MultisigPlugins) > 0 || len(response.Data.TokenVotingPlugins) > 0 || len(response.Data.AddresslistVotingPlugins) > 0 {
		return true, nil
	}

	return false, errors.New("could not find the signer address in any Aragon DAO")
}
