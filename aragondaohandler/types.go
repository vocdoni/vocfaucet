package aragondaohandler

var ValidNetworks = map[string]struct{}{
	"mainnet": {},
	"goerli":  {},
	"sepolia": {},
	"polygon": {},
	"mumbai":  {},
}
var aragonGraphURL = "https://subgraph.satsuma-prod.com/qHR2wGfc5RLi6/aragon/osx-{NETWORK}/version/v1.3.0/api"

type SubgraphMembersResponse struct {
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
