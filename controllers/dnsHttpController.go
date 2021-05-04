package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
	"github.com/gorilla/mux"
	"github.com/txn2/txeh"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/go-playground/validator.v9"
)

func dnsHandlers(r *mux.Router) {

	r.HandleFunc("/api/dns", securityCheck(http.HandlerFunc(getAllDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}/nodes", securityCheck(http.HandlerFunc(getNodeDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}/custom", securityCheck(http.HandlerFunc(getCustomDNS))).Methods("GET")
	r.HandleFunc("/api/dns/adm/{network}", securityCheck(http.HandlerFunc(getDNS))).Methods("GET")
	r.HandleFunc("/api/dns/{network}", securityCheck(http.HandlerFunc(createDNS))).Methods("POST")
	r.HandleFunc("/api/dns/adm/pushdns", securityCheck(http.HandlerFunc(pushDNS))).Methods("POST")
	r.HandleFunc("/api/dns/{network}/{domain}", securityCheck(http.HandlerFunc(deleteDNS))).Methods("DELETE")
	r.HandleFunc("/api/dns/{network}/{domain}", securityCheck(http.HandlerFunc(updateDNS))).Methods("PUT")
}

//Gets all nodes associated with network, including pending nodes
func getNodeDNS(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var dns []models.DNSEntry
        var params = mux.Vars(r)

	dns, err := GetNodeDNS(params["network"])
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }

        //Returns all the nodes in JSON format
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(dns)
}

//Gets all nodes associated with network, including pending nodes
func getAllDNS(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var dns []models.DNSEntry

	networks, err := functions.ListNetworks()
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }

        for _, net := range networks {
                netdns, err := GetDNS(net.NetID)
                if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
                        return
                }
	        dns = append(dns, netdns...)
        }

        //Returns all the nodes in JSON format
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(dns)
}


func GetNodeDNS(network string) ([]models.DNSEntry, error){

        var dns []models.DNSEntry

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"network": network}

        cur, err := collection.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 0}))

        if err != nil {
                return dns, err
        }

        defer cancel()

        for cur.Next(context.TODO()) {

                var entry models.DNSEntry

                err := cur.Decode(&entry)
                if err != nil {
                        return dns, err
                }

                // add item our array of nodes
                dns = append(dns, entry)
        }

        //TODO: Another fatal error we should take care of.
        if err := cur.Err(); err != nil {
                return dns, err
        }

	return dns, err
}

//Gets all nodes associated with network, including pending nodes
func getCustomDNS(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var dns []models.DNSEntry
        var params = mux.Vars(r)

        dns, err := GetCustomDNS(params["network"])
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }

        //Returns all the nodes in JSON format
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(dns)
}

func GetCustomDNS(network string) ([]models.DNSEntry, error){

        var dns []models.DNSEntry

        collection := mongoconn.Client.Database("netmaker").Collection("dns")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"network": network}

        cur, err := collection.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 0}))

        if err != nil {
                return dns, err
        }

        defer cancel()

        for cur.Next(context.TODO()) {

                var entry models.DNSEntry

                err := cur.Decode(&entry)
                if err != nil {
                        return dns, err
                }

                // add item our array of nodes
                dns = append(dns, entry)
        }

        //TODO: Another fatal error we should take care of.
        if err := cur.Err(); err != nil {
                return dns, err
        }

        return dns, err
}

func GetDNSEntryNum(domain string, network string) (int, error){

        num := 0

        entries, err := GetDNS(network)
        if err != nil {
                return 0, err
        }

        for i := 0; i < len(entries); i++ {

                if domain == entries[i].Name {
                        num++
                }
        }

        return num, nil
}

//Gets all nodes associated with network, including pending nodes
func getDNS(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var dns []models.DNSEntry
        var params = mux.Vars(r)

	dns, err := GetDNS(params["network"])
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(dns)
}

func GetDNS(network string) ([]models.DNSEntry, error) {

        var dns []models.DNSEntry
        dns, err := GetNodeDNS(network)
        if err != nil {
                return dns, err
        }
        customdns, err := GetCustomDNS(network)
        if err != nil {
                return dns, err
        }

        dns = append(dns, customdns...)
	return dns, err
}

func createDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var entry models.DNSEntry
        var params = mux.Vars(r)

	//get node from body of request
	_ = json.NewDecoder(r.Body).Decode(&entry)
	entry.Network = params["network"]

	err := ValidateDNSCreate(entry)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	entry, err = CreateDNS(entry)
	if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
        w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entry)
}

func updateDNS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var entry models.DNSEntry

	//start here
	entry, err := GetDNSEntry(params["domain"],params["network"])
	if err != nil {
                returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	var dnschange models.DNSEntry

	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&dnschange)
	if err != nil {
                returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	err = ValidateDNSUpdate(dnschange, entry)

	if err != nil {
                returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	entry, err = UpdateDNS(dnschange, entry)

	if err != nil {
                returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	json.NewEncoder(w).Encode(entry)
}

func deleteDNS(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        // get params
        var params = mux.Vars(r)

        success, err := DeleteDNS(params["domain"], params["network"])

        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        } else if !success {
                returnErrorResponse(w, r, formatError(errors.New("Delete unsuccessful."), "badrequest"))
                return
        }

        json.NewEncoder(w).Encode(params["domain"] + " deleted.")
}

func CreateDNS(entry models.DNSEntry) (models.DNSEntry, error) {

        // connect db
        collection := mongoconn.Client.Database("netmaker").Collection("dns")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // insert our node to the node db.
        _, err := collection.InsertOne(ctx, entry)

        defer cancel()

        return entry, err
}

func GetDNSEntry(domain string, network string) (models.DNSEntry, error) {
        var entry models.DNSEntry

        collection := mongoconn.Client.Database("netmaker").Collection("dns")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"name": domain, "network": network}
        err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&entry)

        defer cancel()

        return entry, err
}

func UpdateDNS(dnschange models.DNSEntry, entry models.DNSEntry) (models.DNSEntry, error) {

        queryDNS := entry.Name

        if dnschange.Name != "" {
                entry.Name = dnschange.Name
        }
        if dnschange.Address != "" {
                entry.Address = dnschange.Address
        }
        //collection := mongoconn.ConnectDB()
        collection := mongoconn.Client.Database("netmaker").Collection("dns")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"name": queryDNS}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"name", entry.Name},
                        {"address", entry.Address},
                }},
        }
        var dnsupdate models.DNSEntry

        errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&dnsupdate)
        if errN != nil {
                fmt.Println("Could not update: ")
                fmt.Println(errN)
        } else {
                fmt.Println("DNS Entry updated successfully.")
        }

        defer cancel()

        return dnsupdate, errN
}

func DeleteDNS(domain string, network string) (bool, error) {

	deleted := false

	collection := mongoconn.Client.Database("netmaker").Collection("dns")

	filter := bson.M{"name": domain,  "network": network}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	result, err := collection.DeleteOne(ctx, filter)

	deletecount := result.DeletedCount

	if deletecount > 0 {
		deleted = true
	}

	defer cancel()

	return deleted, err
}

func pushDNS(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        err := WriteHosts()

        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
	}
        json.NewEncoder(w).Encode("DNS Pushed to CoreDNS")
}


func WriteHosts() error {
	//hostfile, err := txeh.NewHostsDefault()
	hostfile := txeh.Hosts{}
	/*
	if err != nil {
                return err
        }
	*/
	networks, err := functions.ListNetworks()
        if err != nil {
                return err
        }

	for _, net := range networks {
		dns, err := GetDNS(net.NetID)
		if err != nil {
			return err
		}
		for _, entry := range dns {
			hostfile.AddHost(entry.Address, entry.Name+"."+entry.Network)
			if err != nil {
				return err
	                }
		}
	}
	err = hostfile.SaveAs("./config/dnsconfig/netmaker.hosts")
	return err
}

func ValidateDNSCreate(entry models.DNSEntry) error {

	v := validator.New()
        fmt.Println("Validating DNS: " + entry.Name)
        fmt.Println("       Address: " + entry.Address)
        fmt.Println("       Network: " + entry.Network)

	_ = v.RegisterValidation("name_unique", func(fl validator.FieldLevel) bool {
		num, err := GetDNSEntryNum(entry.Name, entry.Network)
		return err == nil && num == 0
	})

	_ = v.RegisterValidation("name_valid", func(fl validator.FieldLevel) bool {
		isvalid := functions.NameInDNSCharSet(entry.Name)
                notEmptyCheck := len(entry.Name) > 0
		return isvalid && notEmptyCheck
	})

	_ = v.RegisterValidation("address_valid", func(fl validator.FieldLevel) bool {
		notEmptyCheck := len(entry.Address) > 0
                isIp := functions.IsIpNet(entry.Address)
		return notEmptyCheck && isIp
	})
        _ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
                _, err := functions.GetParentNetwork(entry.Network)
                return err == nil
        })

	err := v.Struct(entry)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}
	return err
}

func ValidateDNSUpdate(change models.DNSEntry, entry models.DNSEntry) error {

        v := validator.New()

        _ = v.RegisterValidation("name_unique", func(fl validator.FieldLevel) bool {
		goodNum := false
                num, err := GetDNSEntryNum(entry.Name, entry.Network)
		if change.Name != entry.Name {
			goodNum = num == 0
		} else {
                        goodNum = num == 1
		}
		return err == nil && goodNum
        })

        _ = v.RegisterValidation("name_valid", func(fl validator.FieldLevel) bool {
                isvalid := functions.NameInDNSCharSet(entry.Name)
                notEmptyCheck := entry.Name != ""
                return isvalid && notEmptyCheck
        })

        _ = v.RegisterValidation("address_valid", func(fl validator.FieldLevel) bool {
		isValid := true
		if entry.Address != "" {
			isValid = functions.IsIpNet(entry.Address)
		}
		return isValid
        })

        err := v.Struct(entry)

        if err != nil {
                for _, e := range err.(validator.ValidationErrors) {
                        fmt.Println(e)
                }
        }
        return err
}


