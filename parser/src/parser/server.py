import asyncio
import logging
import os

import grpc

from parser.grpc import parser_pb2_grpc
from parser.logging_config import configure
from parser.servicer import ParserServicer

logger = logging.getLogger(__name__)


async def _serve(port: int, num_workers: int) -> None:
    servicer = ParserServicer(num_workers=num_workers)

    server = grpc.aio.server()
    parser_pb2_grpc.add_ParserServiceServicer_to_server(servicer, server)
    server.add_insecure_port(f"[::]:{port}")
    await server.start()
    logger.info(
        "parser grpc server listening",
        extra={"port": port, "workers": num_workers},
    )

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
