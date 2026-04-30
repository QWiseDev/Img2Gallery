import asyncio
import contextlib
import json

from app.admin.repository import active_provider, get_concurrency

from .generator import ImageGenerationError, generate_and_store
from .repository import (
    get_image,
    mark_failed,
    mark_ready,
    mark_running,
    next_queued_jobs,
    queue_counts,
    queue_position,
    reset_running_jobs,
)

_manager_task: asyncio.Task | None = None
_running_tasks: dict[int, asyncio.Task] = {}
_stop_event: asyncio.Event | None = None
JOB_TIMEOUT_SECONDS = 720


async def start_queue_manager() -> None:
    global _manager_task, _stop_event
    reset_running_jobs()
    if _manager_task and not _manager_task.done():
        return
    _stop_event = asyncio.Event()
    _manager_task = asyncio.create_task(queue_loop())


async def stop_queue_manager() -> None:
    if _stop_event:
        _stop_event.set()
    if _manager_task:
        with contextlib.suppress(asyncio.CancelledError):
            await _manager_task
    for task in list(_running_tasks.values()):
        task.cancel()


async def queue_loop() -> None:
    while _stop_event and not _stop_event.is_set():
        cleanup_finished()
        available = max(0, get_concurrency() - len(_running_tasks))
        for image_id in next_queued_jobs(available):
            if image_id in _running_tasks:
                continue
            _running_tasks[image_id] = asyncio.create_task(process_job(image_id))
        await asyncio.sleep(1)


def cleanup_finished() -> None:
    for image_id, task in list(_running_tasks.items()):
        if task.done():
            if not task.cancelled() and task.exception():
                mark_failed(image_id, f"生成任务异常：{task.exception()}")
            _running_tasks.pop(image_id, None)


async def process_job(image_id: int) -> None:
    provider = active_provider()
    if not provider:
        mark_failed(image_id, "未配置可用模型提供商")
        return
    mark_running(image_id, provider["name"], provider["model"])
    image = get_image(image_id, None)
    if not image:
        return
    try:
        image_path = await asyncio.wait_for(
            generate_and_store(image["prompt"], provider),
            timeout=JOB_TIMEOUT_SECONDS,
        )
        mark_ready(image_id, image_path)
    except TimeoutError:
        mark_failed(image_id, f"生成接口超时（超过 {JOB_TIMEOUT_SECONDS} 秒）")
    except ImageGenerationError as exc:
        mark_failed(image_id, str(exc))
    except asyncio.CancelledError:
        mark_failed(image_id, "生成任务已取消")
        raise
    except Exception as exc:
        mark_failed(image_id, f"生成任务异常：{exc}")


async def job_events(image_id: int, viewer_id: int):
    while True:
        image = get_image(image_id, viewer_id)
        if not image:
            yield sse({"status": "missing", "position": None, "queue": queue_counts()})
            return
        payload = {
            "status": image["status"],
            "position": queue_position(image_id),
            "queue": queue_counts(),
            "image": image,
        }
        yield sse(payload)
        if image["status"] in {"ready", "failed"}:
            return
        await asyncio.sleep(1)


def sse(payload: dict) -> str:
    return f"data: {json.dumps(payload, ensure_ascii=False)}\n\n"
