#!/usr/bin/env python3
"""Decode a corpus written by render's TestWriteCorpus with OpenCV.

Usage:
    QR_CORPUS_DIR=/tmp/qrcorpus go test ./render -run TestWriteCorpus
    python3 scripts/opencv_crosscheck.py /tmp/qrcorpus

Every PNG must decode, with OpenCV's QRCodeDetector or its WeChat
detector, to exactly the text in the matching .txt file. This is an
optional external check; the module itself has no Python dependency.
"""
import glob
import os
import sys

import cv2


def main(directory):
    plain = cv2.QRCodeDetector()
    wechat = cv2.wechat_qrcode_WeChatQRCode() if hasattr(cv2, "wechat_qrcode_WeChatQRCode") else None
    ok = failed = 0
    for png in sorted(glob.glob(os.path.join(directory, "*.png"))):
        name = os.path.basename(png)[:-4]
        with open(png[:-4] + ".txt", "rb") as f:
            want = f.read().decode("utf-8")
        img = cv2.imread(png, cv2.IMREAD_GRAYSCALE)
        got = {}
        text, _, _ = plain.detectAndDecode(img)
        got["QRCodeDetector"] = text
        if wechat is not None:
            texts, _ = wechat.detectAndDecode(img)
            got["WeChat"] = texts[0] if texts else ""
        winners = [k for k, v in got.items() if v == want]
        if winners:
            ok += 1
            print(f"ok    {name:20} {', '.join(winners)}")
        else:
            failed += 1
            print(f"FAIL  {name:20} " + "; ".join(f"{k}: {v[:30]!r}" for k, v in got.items()))
    print(f"{ok} decoded, {failed} failed")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else "testdata/corpus"))
