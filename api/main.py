from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from datetime import datetime

app = FastAPI()

class VehicleLog(BaseModel):
    vehicle_plate: str
    entry_date_time: datetime
    exit_date_time: datetime

@app.post("/save_summary")
async def save_summary(log: VehicleLog):
    try:
        entry_time = log.entry_date_time
        exit_time = log.exit_date_time
        duration = exit_time - entry_time

        print(f"Received vehicle log for plate {log.vehicle_plate}")
        print(f"Entry time (UTC): {entry_time}, Exit time (UTC): {exit_time}, Duration: {duration}")

        with open("vehicle_summary.txt", "a") as f:
            f.write(f"{log.vehicle_plate}, {entry_time}, {exit_time}, {duration}\n")

        return {"message": "Summary saved successfully"}

    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to save summary: {str(e)}")
