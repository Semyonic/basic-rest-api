package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Database config
const (
	URI        = "localhost"
	DATABASE   = "store"
	COLLECTION = "products"
)

func ErrorWithJSON(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, "{message: %q}", message)
}

func ResponseWithJSON(w http.ResponseWriter, json []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(json)
}

/*type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}*/

type Product struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	/*	Category *Category*/
	Price string `json:"price"`
}

// Check and create index
func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	c := session.DB("store").C("books")

	index := mgo.Index{
		Key:        []string{"isbn"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err := c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}
}

// Returns all products
func getAllProducts(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB(DATABASE).C(COLLECTION)

		var products []Product
		err := c.Find(bson.M{}).All(&products)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Println("Failed get all products: ", err)
			return
		}

		respBody, err := json.MarshalIndent(products, "", "  ")
		if err != nil {
			log.Fatal(err)
		}

		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}

// Returns given product detail
func getProductById(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		id := pat.Param(r, "id")

		c := session.DB(DATABASE).C(COLLECTION)

		var product Product
		err := c.Find(bson.M{"id": id}).One(&product)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Println("Failed find product: ", err)
			return
		}

		if product.ID == "" {
			ErrorWithJSON(w, "Product not found", http.StatusNotFound)
			return
		}

		respBody, err := json.MarshalIndent(product, "", "  ")
		if err != nil {
			log.Fatal(err)
		}

		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}

// Creates new product from given params
func createProduct(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		var product Product
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&product)
		if err != nil {
			log.Println(decoder)
			ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
			return
		}

		c := session.DB(DATABASE).C(COLLECTION)

		err = c.Insert(product)
		if err != nil {
			if mgo.IsDup(err) {
				ErrorWithJSON(w, "Product with this id already exists", http.StatusBadRequest)
				return
			}

			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Println("Failed insert product: ", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", r.URL.Path+"/"+product.ID)
		w.WriteHeader(http.StatusCreated)
	}
}

// Updates given product with given data
func updateProductById(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		id := pat.Param(r, "id")

		var book Product
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&book)
		if err != nil {
			ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
			return
		}

		c := session.DB(DATABASE).C(COLLECTION)

		err = c.Update(bson.M{"id": id}, &book)
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				log.Println("Failed update product: ", err)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "Product not found", http.StatusNotFound)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// Deletes given product by given id
func deleteProductById(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		id := pat.Param(r, "id")

		c := session.DB(DATABASE).C(COLLECTION)

		err := c.Remove(bson.M{"id": id})
		if err != nil {
			switch err {
			default:
				ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
				log.Println("Failed delete product: ", err)
				return
			case mgo.ErrNotFound:
				ErrorWithJSON(w, "Product not found", http.StatusNotFound)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func api() {

	// Create mongodb connection session
	session, err := mgo.Dial(URI)
	if err != nil {
		panic(err)
	}

	// Delay close event
	defer session.Close()

	session.SetMode(mgo.Primary, true)

	// Before querying, check that indexes exists
	ensureIndex(session)

	// Route handling
	mux := goji.NewMux()
	mux.HandleFunc(pat.Get("/products"), getAllProducts(session))
	mux.HandleFunc(pat.Post("/products"), createProduct(session))
	mux.HandleFunc(pat.Get("/products/:{id}"), getProductById(session))
	mux.HandleFunc(pat.Put("/products/:{id}"), updateProductById(session))
	mux.HandleFunc(pat.Delete("/products/:{id}"), deleteProductById(session))

	// Server
	http.ListenAndServe("localhost:8080", mux)
}
