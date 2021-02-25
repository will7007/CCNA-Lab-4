package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
)

var databaseMutex = &sync.RWMutex{}
//A mutex is like the "talking stick" of programming: if two processes would like to access something that could change
//based on the actions of another process, then letting them just access it whenever is dangerous because that value might
//be in the middle of being updated and it will be out of date when it's read.
//It's better to only let one process access a critical section of code at a time, and a mutex functions like a lock that
//must be acquired before a thread (or in this case, a goroutine) can continue to do its stuff. The other threads will
//politely wait until the thread with the mutex is done with the data and puts the mutex back (unlocks it)--the next scheduled
//thread (which could be any of them, at random) which is waiting on the mutex will grab it and get to do its stuff while
//the other threads have to wait.
//This mutex is a RWMutex, which means that it has a general locking ability (to lock out reading and writing) and a read-specific
//lock which lets other threads acquire a RLock at the same time and read, but ensures no threads try to write while
//threads are reading.

func main() {
	db := database{"shoes": 50, "socks": 5} //make a new database for us to access
	http.HandleFunc("/list", db.list) //make it so when the user puts /list at the end of the URL, we run db.list()
	http.HandleFunc("/create", db.create) //same for /create and db.create()
	http.HandleFunc("/price", db.price)
	http.HandleFunc("/update", db.update)
	http.HandleFunc("/delete", db.delete)
	log.Fatal(http.ListenAndServe("localhost:8000", nil)) //listen to our local address (ourself) on port 8000,
	//and if we have an issue then log it as a fatal error
}

type dollars float32

func (d dollars) String() string { return fmt.Sprintf("$%.2f", d) }

type database map[string]dollars

//send over the contents of the database map to the asking client: keys and values alike
func (db database) list(w http.ResponseWriter, req *http.Request) {
	databaseMutex.RLock() //I call dibs on the critical section, but I will only read from it, so if you'd like to read from it as well then go ahead
	for item, price := range db { //for each item (key) and price (value) in the database, send out the item and price
		fmt.Fprintf(w, "%s: %s\n", item, price)
	}
	databaseMutex.RUnlock() //okay all done reading, other threads can write to the database now
}

//sticks a value inside the database map if it isn't already in there
func (db database) create(w http.ResponseWriter, req *http.Request) {
	item := req.URL.Query().Get("item")
	price := req.URL.Query().Get("price")
	databaseMutex.Lock() //I call dibs on the critical section! Other threads which need to Lock() will just have to wait
	if existingPrice, ok := db[item]; ok { //if the item's already in the database then too bad, they should have updated it
		//log.Println("Item",item,"already existed when client tried to create")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprintf(w, "Item %s already exists with price %s\n", item, existingPrice)
	} else {
		if dollarsFloat, err := strconv.ParseFloat(price,32); err == nil {
			db[item] = dollars(dollarsFloat)
			fmt.Fprintf(w, "Item %s created with price %.2f\n", item, dollarsFloat)
		} else {
			w.WriteHeader(http.StatusNotAcceptable)
			//log.Println("Invalid value",price,"provided by client for item",item)
			fmt.Fprintf(w, "Provided price is invalid\n")
		}
	}
	databaseMutex.Unlock() //okay I'm done, I'll release the mutex so that another thread can go
}

//read from the database map and send the price over to the asking client
func (db database) price(w http.ResponseWriter, req *http.Request) {
	item := req.URL.Query().Get("item") //get the thing that we put in the URL with price?item=VALUE
	databaseMutex.RLock()
	if price, ok := db[item]; ok { //if the item the URL asked for exists, time to send it back
		fmt.Fprintf(w, "%s\n", price)
	} else {
		w.WriteHeader(http.StatusNotFound) // 404 if we didn't find the item in the database
		fmt.Fprintf(w, "no such item: %q\n", item)
	}
	databaseMutex.RUnlock()
}

//update a value in the database map if the value is present in the database
func (db database) update(w http.ResponseWriter, req *http.Request) {
	item := req.URL.Query().Get("item")
	price := req.URL.Query().Get("price")
	databaseMutex.Lock()
	if _, ok := db[item]; ok {
		if dollarsFloat, err := strconv.ParseFloat(price,32); err == nil {
			db[item] = dollars(dollarsFloat)
			fmt.Fprintf(w, "Item %s updated with price %.2f\n", item, dollarsFloat)
			//time.Sleep(5*time.Second) //uncomment and try to run /update and something else to test mutex
		} else {
			w.WriteHeader(http.StatusNotAcceptable)
			//log.Println("Invalid value",price,"provided by client for updating item",item)
			fmt.Fprintf(w, "Provided price is invalid\n")
		}
	} else {
		w.WriteHeader(http.StatusNotFound) // 404 if we didn't find the item in the database
		fmt.Fprintf(w, "no such item: %q\n", item)
	}
	databaseMutex.Unlock()
}

//remove an item from the database map
func (db database) delete(w http.ResponseWriter, req *http.Request) {
	item := req.URL.Query().Get("item")
	databaseMutex.Lock()
	if _, ok := db[item]; ok {
		delete(db,item)
		fmt.Fprintf(w, "%q deleted\n", item)
	} else {
		w.WriteHeader(http.StatusNotFound) // 404 if we didn't find the item in the database
		fmt.Fprintf(w, "no such item: %q\n", item)
	}
	databaseMutex.Unlock()
}
