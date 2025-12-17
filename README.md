# metahub-api

## Run the API 'by hand'

```
docker build -t qnib/metahub-api .
docker run -ti --rm -p 8080:8080 \
   -e NEO_URL=bolt://localhost:7687 \
   qnib/metahub-api
```

## More Docs
