package eda

import (
	"encoding/json"
	"os"
	"sirius.local/food/internal/domain"
	"testing"
)

func TestOrderPaidMessageContract(t *testing.T) {
	b, e := os.ReadFile("../../contracts/OrderPaid.json")
	if e != nil {
		t.Fatal(e)
	}
	var expected Event
	if e = json.Unmarshal(b, &expected); e != nil {
		t.Fatal(e)
	}
	actual, e := json.Marshal(Event{ID: "contract-example", Type: "order.paid", OrderID: 42, UserID: 7, Amount: domain.Money(15000), CorrelationID: "contract-order-42"})
	if e != nil {
		t.Fatal(e)
	}
	var consumer Event
	if e = json.Unmarshal(actual, &consumer); e != nil {
		t.Fatal(e)
	}
	if consumer != expected {
		t.Fatalf("provider/consumer mismatch: %+v %+v", consumer, expected)
	}
	if !validType("kitchen", consumer.Type) {
		t.Fatal("kitchen refuses published event")
	}
}
