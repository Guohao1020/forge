from __future__ import annotations
import asyncio
import logging
from temporalio.client import Client
from temporalio.worker import Worker
from src.config import settings
from src.activities.analyze import analyze_requirement_activity
from src.activities.plan import plan_task_activity
from src.activities.generate import generate_code_activity
from src.activities.review import review_code_activity
from src.activities.test_writing import generate_test_cases_activity
from src.activities.profile import scan_project_profile_activity

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(name)s: %(message)s")
logger = logging.getLogger(__name__)

async def main() -> None:
    logger.info(f"Connecting to Temporal at {settings.temporal_host}...")
    client = await Client.connect(settings.temporal_host, namespace=settings.temporal_namespace)
    logger.info(f"Starting AI Worker on task queue '{settings.task_queue}'")
    worker = Worker(
        client,
        task_queue=settings.task_queue,
        activities=[
            analyze_requirement_activity,
            plan_task_activity,
            generate_code_activity,
            review_code_activity,
            generate_test_cases_activity,
            scan_project_profile_activity,
        ],
    )
    logger.info("AI Worker started. Waiting for activities...")
    await worker.run()

if __name__ == "__main__":
    asyncio.run(main())
