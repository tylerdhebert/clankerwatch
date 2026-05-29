//go:build ignore

// Seed a storefront SQLite database for local clankerwatch testing.
// Usage: go run ./scripts/seed-storefront.go [output-path]
package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	numCategories = 18
	numProducts   = 220
	numCustomers  = 200
	numOrders     = 240
)

func main() {
	out := filepath.Join("testdata", "storefront.sqlite")
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fatal(err)
	}
	_ = os.Remove(out)

	db, err := sql.Open("sqlite", out)
	if err != nil {
		fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		fatal(err)
	}

	schema := `
CREATE TABLE categories (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE products (
  id INTEGER PRIMARY KEY,
  category_id INTEGER NOT NULL REFERENCES categories(id),
  sku TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  price_cents INTEGER NOT NULL CHECK (price_cents >= 0),
  stock_qty INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE customers (
  id INTEGER PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  full_name TEXT NOT NULL,
  city TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE orders (
  id INTEGER PRIMARY KEY,
  customer_id INTEGER NOT NULL REFERENCES customers(id),
  status TEXT NOT NULL CHECK (status IN ('pending','paid','shipped','cancelled')),
  placed_at TEXT NOT NULL,
  total_cents INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE order_items (
  id INTEGER PRIMARY KEY,
  order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  product_id INTEGER NOT NULL REFERENCES products(id),
  quantity INTEGER NOT NULL CHECK (quantity > 0),
  unit_price_cents INTEGER NOT NULL CHECK (unit_price_cents >= 0),
  UNIQUE (order_id, product_id)
);
CREATE INDEX idx_products_category ON products(category_id);
CREATE INDEX idx_orders_customer ON orders(customer_id);
CREATE INDEX idx_order_items_order ON order_items(order_id);
CREATE INDEX idx_order_items_product ON order_items(product_id);
`
	if _, err := db.Exec(schema); err != nil {
		fatal(err)
	}

	rng := rand.New(rand.NewSource(42))
	categoryNames := []string{
		"Apparel", "Footwear", "Accessories", "Home", "Kitchen",
		"Electronics", "Books", "Toys", "Garden", "Sports",
		"Beauty", "Pet", "Office", "Automotive", "Crafts",
		"Music", "Outdoor", "Grocery",
	}
	adjectives := []string{"Classic", "Premium", "Budget", "Artisan", "Compact", "Deluxe", "Eco", "Vintage", "Modern", "Rustic"}
	nouns := []string{"Mug", "Lamp", "Chair", "Backpack", "Jacket", "Sneaker", "Notebook", "Speaker", "Blanket", "Bottle", "Watch", "Planter", "Desk", "Pillow", "Toolkit"}
	cities := []string{"Austin", "Portland", "Denver", "Chicago", "Boston", "Seattle", "Miami", "Phoenix", "Atlanta", "Minneapolis"}
	first := []string{"Alex", "Jordan", "Taylor", "Morgan", "Casey", "Riley", "Quinn", "Avery", "Jamie", "Drew", "Sam", "Blake", "Cameron", "Hayden", "Logan"}
	last := []string{"Lee", "Patel", "Garcia", "Kim", "Nguyen", "Brown", "Davis", "Martinez", "Wilson", "Anderson", "Thomas", "Jackson", "White", "Harris", "Clark"}
	statuses := []string{"pending", "paid", "shipped", "cancelled"}

	tx, err := db.Begin()
	if err != nil {
		fatal(err)
	}
	defer tx.Rollback()

	catStmt, err := tx.Prepare(`INSERT INTO categories (id, name) VALUES (?, ?)`)
	if err != nil {
		fatal(err)
	}
	defer catStmt.Close()
	for i := 1; i <= numCategories; i++ {
		name := categoryNames[i-1]
		if _, err := catStmt.Exec(i, name); err != nil {
			fatal(err)
		}
	}

	prodStmt, err := tx.Prepare(`INSERT INTO products (id, category_id, sku, name, price_cents, stock_qty) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		fatal(err)
	}
	defer prodStmt.Close()
	for i := 1; i <= numProducts; i++ {
		cat := 1 + rng.Intn(numCategories)
		name := fmt.Sprintf("%s %s", adjectives[rng.Intn(len(adjectives))], nouns[rng.Intn(len(nouns))])
		sku := fmt.Sprintf("SKU-%05d", i)
		price := 299 + rng.Intn(25000)
		stock := rng.Intn(500)
		if _, err := prodStmt.Exec(i, cat, sku, name, price, stock); err != nil {
			fatal(err)
		}
	}

	custStmt, err := tx.Prepare(`INSERT INTO customers (id, email, full_name, city, created_at) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		fatal(err)
	}
	defer custStmt.Close()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= numCustomers; i++ {
		fn := first[rng.Intn(len(first))]
		ln := last[rng.Intn(len(last))]
		email := fmt.Sprintf("%s.%s%d@example.com", strings.ToLower(fn), strings.ToLower(ln), i)
		city := cities[rng.Intn(len(cities))]
		created := base.Add(time.Duration(rng.Intn(500)) * 24 * time.Hour).Format(time.RFC3339)
		if _, err := custStmt.Exec(i, email, fn+" "+ln, city, created); err != nil {
			fatal(err)
		}
	}

	orderStmt, err := tx.Prepare(`INSERT INTO orders (id, customer_id, status, placed_at, total_cents) VALUES (?, ?, ?, ?, 0)`)
	if err != nil {
		fatal(err)
	}
	defer orderStmt.Close()
	itemStmt, err := tx.Prepare(`INSERT INTO order_items (order_id, product_id, quantity, unit_price_cents) VALUES (?, ?, ?, ?)`)
	if err != nil {
		fatal(err)
	}
	defer itemStmt.Close()
	updateTotal, err := tx.Prepare(`UPDATE orders SET total_cents = ? WHERE id = ?`)
	if err != nil {
		fatal(err)
	}
	defer updateTotal.Close()

	itemID := 0
	for o := 1; o <= numOrders; o++ {
		cust := 1 + rng.Intn(numCustomers)
		status := statuses[rng.Intn(len(statuses))]
		placed := base.Add(time.Duration(200+rng.Intn(400)) * 24 * time.Hour).Format(time.RFC3339)
		if _, err := orderStmt.Exec(o, cust, status, placed); err != nil {
			fatal(err)
		}
		lineCount := 1 + rng.Intn(4)
		used := map[int]bool{}
		total := 0
		for line := 0; line < lineCount; line++ {
			pid := 1 + rng.Intn(numProducts)
			if used[pid] {
				continue
			}
			used[pid] = true
			qty := 1 + rng.Intn(3)
			var unit int
			if err := tx.QueryRow(`SELECT price_cents FROM products WHERE id = ?`, pid).Scan(&unit); err != nil {
				fatal(err)
			}
			itemID++
			if _, err := itemStmt.Exec(o, pid, qty, unit); err != nil {
				fatal(err)
			}
			total += unit * qty
		}
		if len(used) == 0 {
			pid := 1 + rng.Intn(numProducts)
			var unit int
			if err := tx.QueryRow(`SELECT price_cents FROM products WHERE id = ?`, pid).Scan(&unit); err != nil {
				fatal(err)
			}
			itemID++
			if _, err := itemStmt.Exec(o, pid, 1, unit); err != nil {
				fatal(err)
			}
			total = unit
		}
		if _, err := updateTotal.Exec(total, o); err != nil {
			fatal(err)
		}
	}

	if err := tx.Commit(); err != nil {
		fatal(err)
	}

	abs, _ := filepath.Abs(out)
	for _, q := range []struct {
		label, sql string
	}{
		{"categories", "SELECT COUNT(*) FROM categories"},
		{"products", "SELECT COUNT(*) FROM products"},
		{"customers", "SELECT COUNT(*) FROM customers"},
		{"orders", "SELECT COUNT(*) FROM orders"},
		{"order_items", "SELECT COUNT(*) FROM order_items"},
	} {
		var n int
		if err := db.QueryRow(q.sql).Scan(&n); err != nil {
			fatal(err)
		}
		fmt.Printf("%s: %d rows\n", q.label, n)
	}
	fmt.Printf("\nWrote %s\n", abs)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
