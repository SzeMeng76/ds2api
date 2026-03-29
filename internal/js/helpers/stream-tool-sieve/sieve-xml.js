'use strict';
const { parseToolCalls } = require('./parse');
const {
  XML_TOOL_OPENING_TAGS,
  XML_TOOL_CLOSING_TAGS,
} = require('./tool-keywords');

function consumeXMLToolCapture(captured, toolNames, trimWrappingJSONFence) {
  const lower = captured.toLowerCase();
  let openIdx = -1;
  for (const tag of XML_TOOL_OPENING_TAGS) {
    const idx = lower.indexOf(tag);
    if (idx >= 0 && (openIdx < 0 || idx < openIdx)) {
      openIdx = idx;
    }
  }
  if (openIdx < 0) {
    return { ready: false, prefix: '', calls: [], suffix: '' };
  }
  let closeIdx = -1;
  for (const tag of XML_TOOL_CLOSING_TAGS) {
    const idx = lower.indexOf(tag, openIdx);
    if (idx >= 0) {
      const absEnd = idx + tag.length;
      if (closeIdx < 0 || absEnd > closeIdx) {
        closeIdx = absEnd;
      }
    }
  }
  if (closeIdx <= 0) {
    return { ready: false, prefix: '', calls: [], suffix: '' };
  }
  const xmlBlock = captured.slice(openIdx, closeIdx);
  let prefixPart = captured.slice(0, openIdx);
  let suffixPart = captured.slice(closeIdx);
  const parsed = parseToolCalls(xmlBlock, toolNames);
  if (Array.isArray(parsed) && parsed.length > 0) {
    const trimmedFence = trimWrappingJSONFence(prefixPart, suffixPart);
    return {
      ready: true,
      prefix: trimmedFence.prefix,
      calls: parsed,
      suffix: trimmedFence.suffix,
    };
  }
  return { ready: true, prefix: prefixPart, calls: [], suffix: suffixPart };
}

function hasOpenXMLToolTag(captured) {
  const lower = captured.toLowerCase();
  for (const tag of XML_TOOL_OPENING_TAGS) {
    if (lower.includes(tag)) {
      let hasClosed = false;
      for (const ct of XML_TOOL_CLOSING_TAGS) {
        if (lower.includes(ct)) {
          hasClosed = true;
          break;
        }
      }
      if (!hasClosed) {
        return true;
      }
    }
  }
  return false;
}

function findPartialXMLToolTagStart(s) {
  const lastLT = s.lastIndexOf('<');
  if (lastLT < 0) {
    return -1;
  }
  const tail = s.slice(lastLT);
  if (tail.includes('>')) {
    return -1;
  }
  const lowerTail = tail.toLowerCase();
  for (const tag of XML_TOOL_OPENING_TAGS) {
    const tagWithLT = tag.startsWith('<') ? tag : '<' + tag;
    if (tagWithLT.startsWith(lowerTail)) {
      return lastLT;
    }
  }
  return -1;
}

module.exports = {
  consumeXMLToolCapture,
  hasOpenXMLToolTag,
  findPartialXMLToolTagStart,
};
