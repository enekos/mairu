# Retrieval Evaluation

## Case: auth-mem
Query: "JWT authentication tokens"
Domain: memory | TopK: 5

### Results
1. eval-mem-auth-1 [EXPECTED]

MRR: 1.000 | Recall@5: 1.000

## Case: testing-mem
Query: "how to write Go tests"
Domain: memory | TopK: 5

### Results

MRR: 0.000 | Recall@5: 0.000

## Case: db-mem
Query: "database connection pool"
Domain: memory | TopK: 5

### Results
1. eval-mem-db-1 [EXPECTED]

MRR: 1.000 | Recall@5: 1.000

## Case: auth-skill
Query: "token validation and authentication"
Domain: skill | TopK: 5

### Results
1. eval-skill-auth-1 [EXPECTED]

MRR: 1.000 | Recall@5: 1.000

## Case: auth-ctx
Query: "authentication architecture"
Domain: context | TopK: 5

### Results
1. contextfs://eval-approved-test/backend [EXPECTED]
2. contextfs://eval-approved-test/backend/auth [EXPECTED]

MRR: 1.000 | Recall@5: 1.000

## Case: scraper-mem
Query: "web page scraping dynamic content puppeteer markdown"
Domain: memory | TopK: 5

### Results
1. eval-mem-scraper-1 [EXPECTED]

MRR: 1.000 | Recall@5: 0.500

---
## Summary
MRR: 0.833 | Recall@5: 0.750 | Precision@5: 0.200 | NDCG@5: 0.769 | MAP: 0.750
