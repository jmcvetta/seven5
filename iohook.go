package seven5

import (
	"net/http"
	"io"
	"fmt"
	"os"
	"errors"
	"reflect"
)

//IOHook is an interface provided as a convenience to those who want to override
//the behavior of some aspect of reading and writing web content.  This can be used
//to change the format of http input/output (such as ignoring parts of the body
//content, changing how cookies are used, etc)
//or to change the parameters passed to rest resources in the PBundle (for example to
//modify query parameter arguments programmatically).
type IOHook interface {
	SendHook(d *restObj, w http.ResponseWriter, pb PBundle, i interface{}, location string)
	BundleHook(w http.ResponseWriter, r *http.Request, sm SessionManager) (PBundle, error)
	BodyHook(r *http.Request, obj *restObj) (interface{}, error) 
	CookieMapper() CookieMapper
}

//RawIOHook is the default implementation of the IOHook used by the RawDispatcher.
type RawIOHook struct {
	Dec Decoder
	Enc Encoder
	CookieMap CookieMapper
}

//CookieMapper is exposed because other parts of the system may need access to the 
//cookie mapper to allow them to manipulate cookies.  This allows centralization of
//all cookie handling in one type.
func (self *RawIOHook) CookieMapper() CookieMapper {
	return self.CookieMap
}

//NewRawIOHook returns a new RawIOHook ptr with the decoder and encoder provided. This 
//object needs a cookie mapper because setting and reading cookies is IO to the client!
//You can provided your own encoder and decoder pair if you wish to just change the
//format of the encoding used when marshalling and unmarshalling wire types (from
//json to xml, for example).  
func NewRawIOHook(d Decoder, e Encoder, c CookieMapper) *RawIOHook{
	return &RawIOHook{Dec: d, Enc: e, CookieMap: c}
}

//BodyHook is called to create a wire object of the appopriate type and fill in the values
//in that object from the request body.  BodyHook calls the decoder provided at creation time
//take the bytes provided by the body and initialize the object that is ultimately returned.
func (self *RawIOHook) BodyHook(r *http.Request, obj *restObj) (interface{}, error) {
	limitedData := make([]byte, MAX_FORM_SIZE)
	curr := 0
	gotEof := false
	for curr < len(limitedData) {
		n, err := r.Body.Read(limitedData[curr:])
		if err != nil && err == io.EOF {
			gotEof = true
			break
		}
		if err != nil {
			return nil, err
		}
		curr += n
	}
	//if curr==0 then we are done because there is no body
	if curr == 0 {
		return nil, nil
	}
	if !gotEof {
		return nil, errors.New(fmt.Sprintf("Body is too large! max is %d", MAX_FORM_SIZE))
	}
	//we have a body of data, need to decode it... first allocate one
	wireObj := reflect.New(obj.t)
	if err := self.Dec.Decode(limitedData[:curr], wireObj.Interface()); err != nil {
		return nil, err
	}

	return wireObj.Interface(), nil
}


//BundleHook is called to create the bundle of parameters from the request. It often will be
//using cookies and sessions to compute the bundle.  Note that the ResponseWriter is passed
//here but the BundleHook _must_ be careful to not force it out the server--it should only
//add headers.
func (self *RawIOHook) BundleHook(w http.ResponseWriter, r *http.Request, sm SessionManager) (PBundle, error) {
	var session Session
	if self.CookieMap != nil {
		var err error
		id, err := self.CookieMap.Value(r)
		if err != nil && err != NO_SUCH_COOKIE {
			return nil, err
		}
		var find_err error
		if sm != nil {
			session, find_err = sm.Find(id)
			if find_err != nil {
				return nil, find_err
			}
			if session == nil && err != NO_SUCH_COOKIE {
				self.CookieMap.RemoveCookie(w)
			}
		}
	}
	pb, err := NewSimplePBundle(r, session)
	if err != nil {
		return nil, err
	}
	return pb, nil
}


//SendHook is called to encode and write the object provided onto the output via the response
//writer.  The last parameter if not "" is assumed to be a location header.  If the location
//parameter is provided, then the response code is "Created" otherwise "OK" is returned.
//SendHook calls the encoder for the encoding of the object into a sequence of bytes for transmission.
//If the pb is not null, then the SendHook should examine it for outgoing headers, trailers, and
//transmit them.
func (self *RawIOHook) SendHook(d *restObj, w http.ResponseWriter, pb PBundle, i interface{}, location string) {
	if err := self.verifyReturnType(d, i); err != nil {
		http.Error(w, fmt.Sprintf("%s", err), http.StatusExpectationFailed)
		return
	}
	encoded, err := self.Enc.Encode(i, true)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to encode: %s", err), http.StatusInternalServerError)
		return
	}
	for _, k:=range pb.ReturnHeaders() {
		w.Header().Add(k,pb.ReturnHeader(k))
	}
	w.Header().Add("Content-Type", "text/json")
	if location != "" {
		w.Header().Add("Location", location)
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_, err = w.Write([]byte(encoded))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to write to client connection: %s\n", err)
	}
}

func (self *RawIOHook) verifyReturnType(obj *restObj, w interface{}) error {
	if w == nil {
		return nil
	}
	p := reflect.TypeOf(w)
	var e reflect.Type
	if p.Kind() != reflect.Ptr {
		//could be a slice of these pointers
		if p.Kind() != reflect.Slice {
			return errors.New(fmt.Sprintf("Marshalling problem: expected a pointer/slice type but got a %v", p.Kind()))
		}
		s:=reflect.ValueOf(w)
		//you can send an _empty_ slice of anything
		if s.Len()==0 {
			return nil
		}
		v:=s.Index(0)
		p=reflect.TypeOf(v)
		if v.CanInterface() {
			i:=v.Interface()
			p=reflect.TypeOf(i)
		} 
		if p.Kind() != reflect.Ptr {
			return errors.New(fmt.Sprintf("Marshalling problem: expected a ptr but got %v", p.Kind()))
		}
	} 
	e=p.Elem()
	if e != obj.t {
		return errors.New(fmt.Sprintf("Marshalling problem: expected pointer to %v but got pointer to %v",
			obj.t, e))
	}
	return nil
}

