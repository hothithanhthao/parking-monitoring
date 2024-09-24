from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from datetime import datetime
from prometheus_fastapi_instrumentator import Instrumentator

app = FastAPI()

class VehicleLog(BaseModel):
    vehicle_plate: str
    entry_date_time: datetime
    exit_date_time: datetime

@app.post("/save_summary")
async def save_summary(log: VehicleLog):
    try:
        duration = log.exit_date_time - log.entry_date_time
        print(f"Received vehicle log for plate {log.vehicle_plate}")
        print(f"Entry time (UTC): {log.entry_date_time}, Exit time (UTC): {log.exit_date_time}, Duration: {duration}")
        with open("vehicle_summary.txt", "a") as f:
            f.write(f"{log.vehicle_plate}, {log.entry_date_time}, {log.exit_date_time}, {duration}\n")
        return {"message": "Summary saved successfully"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to save summary: {str(e)}")

Instrumentator().instrument(app).expose(app, endpoint="/metrics")
