import asyncio
import logging
import os

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from parser.formats.converter import warmup
from parser.grpc import parser_pb2_grpc
from parser.logging_config import configure
from parser.servicer import ParserServicer

logger = logging.getLogger(__name__)


async def _serve(port: int, num_workers: int) -> None:
    servicer = ParserServicer(num_workers=num_workers)
    health_servicer = health.HealthServicer()

    server = grpc.aio.server()
    parser_pb2_grpc.add_ParserServiceServicer_to_server(servicer, server)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)
    server.add_insecure_port(f"[::]:{port}")
    await server.start()
    logger.info(
        "parser grpc server listening",
        extra={"port": port, "workers": num_workers},
    )

    loop = asyncio.get_running_loop()
    logger.info("parser warming up models")
    try:
        await loop.run_in_executor(None, warmup)
        health_servicer.set("", health_pb2.HealthCheckResponse.SERVING)
        logger.info("parser ready")
    except Exception:
        logger.exception("model warmup failed")
        health_servicer.set("", health_pb2.HealthCheckResponse.NOT_SERVING)

    try:
        await server.wait_for_termination()
    finally:
        logger.info("parser grpc server shutting down")
        servicer.shutdown()


def serve() -> None:
    configure(os.environ.get("LOG_LEVEL"))
    port = int(os.environ.get("GRPC_PORT", "50051"))
    num_workers = int(os.environ.get("NUM_WORKERS", "2"))
    asyncio.run(_serve(port, num_workers))
