from fastapi import FastAPI

app = FastAPI(title="API Gateway")

@app.get("/")
def read_root():
    return {"message": "API Gateway is running"}
