import os

from docling_core.types.doc.base import ImageRefMode


def bool_env(var: str, default: bool) -> bool:
    val = os.environ.get(var)
    if val is None:
        return default
    return val.lower() in ("1", "true", "yes")


def image_mode(env_var: str) -> ImageRefMode:
    val = os.environ.get(env_var, "placeholder").lower()
    return {"embedded": ImageRefMode.EMBEDDED, "referenced": ImageRefMode.REFERENCED}.get(
        val, ImageRefMode.PLACEHOLDER
    )
