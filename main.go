package main

type Config struct {
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	Uri       string     `json:"uri"`
	Template  string     `json:"template"`
	Variables []Variable `json:"variables"`
}

type Variable struct {
	Name   string `json:"name"`
	EnvVar string `json:"env_var"`
	Value  string `json:"value"`
}

func main() {

}
