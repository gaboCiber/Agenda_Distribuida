from fastapi import FastAPI

app = FastAPI(title="Notifications Service")

@app.get("/")
def read_root():
    return {"message": "Notifications Service is running"}
