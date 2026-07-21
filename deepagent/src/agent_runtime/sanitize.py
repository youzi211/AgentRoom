from __future__ import annotations


OPEN_TAGS = ("<think>", "<thinking>")


class ThinkBlockFilter:
    """Incrementally removes private reasoning tags, including split tags."""

    def __init__(self) -> None:
        self._buffer = ""
        self._inside: str | None = None

    def feed(self, text: str) -> str:
        self._buffer += text
        visible: list[str] = []
        while self._buffer:
            lowered = self._buffer.lower()
            if self._inside is not None:
                close = f"</{self._inside}>"
                index = lowered.find(close)
                if index < 0:
                    self._buffer = self._buffer[-(len(close) - 1) :]
                    break
                self._buffer = self._buffer[index + len(close) :]
                self._inside = None
                continue

            matches = [(lowered.find(tag), tag) for tag in OPEN_TAGS]
            matches = [(index, tag) for index, tag in matches if index >= 0]
            if matches:
                index, tag = min(matches, key=lambda item: item[0])
                visible.append(self._buffer[:index])
                self._buffer = self._buffer[index + len(tag) :]
                self._inside = tag[1:-1]
                continue

            keep = _possible_open_tag_suffix(lowered)
            if keep:
                visible.append(self._buffer[:-keep])
                self._buffer = self._buffer[-keep:]
            else:
                visible.append(self._buffer)
                self._buffer = ""
            break
        return "".join(visible)

    def finish(self) -> str:
        if self._inside is not None:
            self._buffer = ""
            return ""
        remaining = self._buffer
        self._buffer = ""
        return remaining


def _possible_open_tag_suffix(text: str) -> int:
    for length in range(min(len(text), max(map(len, OPEN_TAGS)) - 1), 0, -1):
        suffix = text[-length:]
        if any(tag.startswith(suffix) for tag in OPEN_TAGS):
            return length
    return 0
