export function useScreenshot({ chatOpen, readerRef, addImageToChat, copyImage }) {
  async function deliverScreenshot(imageDataUrl) {
    const results = [
      { label: '复制截图到剪贴板失败', task: copyImage },
    ];
    if (chatOpen.value) {
      results.push({ label: '添加截图到聊天失败', task: addImageToChat });
    }

    const outcomes = await Promise.allSettled(results.map(async ({ task }) => task(imageDataUrl)));
    const errors = outcomes.flatMap((outcome, index) => outcome.status === 'rejected'
      ? [`${results[index].label}：${outcome.reason?.message || outcome.reason}`]
      : []);
    if (errors.length) alert(errors.join('\n'));
  }

  async function captureCurrentPage() {
    const imageDataUrl = await readerRef.value?.captureCurrentPage();
    if (!imageDataUrl) {
      alert('无法截取当前页');
      return;
    }
    await deliverScreenshot(imageDataUrl);
  }

  async function startAreaSelection() {
    const imageDataUrl = await readerRef.value?.startAreaSelection();
    if (imageDataUrl) await deliverScreenshot(imageDataUrl);
  }

  return { captureCurrentPage, startAreaSelection };
}
