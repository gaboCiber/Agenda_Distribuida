from fastapi import FastAPI

app = FastAPI(title="Users Service")

@app.get("/")
def read_root():
    return {"message": "Users Service is running"}
