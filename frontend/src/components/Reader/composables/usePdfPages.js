import { ref,onMounted, onBeforeUnmount, reactive } from "vue";


// PDF 页面管理 以及 渲染相关
export function usePdfPages(
    pdfDoc,
    totalPages,
    readerContainer,
    pdfCanvas,
) {
    const pageCanvasRefs = {};
    const pageTextLayerRefs = {};
    const pageContainerRefs = {};

    const renderingPages = ref(new Set()); // 正在渲染的页码，避免重复渲染
    const currentPages = ref(new Set()); // 当前可见的页码

    const pageWidth = ref(600); // 统一管理页面宽度，所有缩放路径都通过修改它来触发重新渲染

    let pageObserver = null;
    let renderTimeout = null;

    const RENDER_THROTTLE = 200;

    function RegisterPageRefs(pageNum, els) {
        if (els) {
            pageContainerRefs[pageNum] = els.container;
            pageCanvasRefs[pageNum] = els.canvas;
            pageTextLayerRefs[pageNum] = els.textLayer;
            
        } else {
            delete pageContainerRefs[pageNum];
            delete pageCanvasRefs[pageNum];
            delete pageTextLayerRefs[pageNum];
        }
    }

    const ClearPages = async () => { 
        console.log('Clearing pages...');
        currentPages.value.clear();
        renderingPages.value.clear();

        // 清理旧的 refs
        Object.keys(pageCanvasRefs).forEach(key => delete pageCanvasRefs[key]);
        Object.keys(pageTextLayerRefs).forEach(key => delete pageTextLayerRefs[key]);
        Object.keys(pageContainerRefs).forEach(key => delete pageContainerRefs[key]);
    };

    // 计算页面的 viewport 和 outputScale
    const computePageViewport = (page) => {
        const pageWidthInPoints = page.view[2] - page.view[0];  // PDF 页面宽度（单位：点, 也就是1/72英寸）
        const scale = pageWidth.value / pageWidthInPoints; // 每个点对应的像素数，pageWidth是我们设定的页面宽度（单位：像素）
        // 根据我们提供的scale计算viewport，viewport.width应该等于pageWidth.value
        //  viewport包含了页面的尺寸和变换信息，比如页面的宽高（单位：像素）
        const viewport = page.getViewport({ scale });
        // 高分辨率渲染（考虑 devicePixelRatio）
        // devicePixelRatio 是一个表示设备像素与CSS像素之间关系的值。对于普通屏幕，这个值通常是1；
        // 对于高DPI屏幕（如Retina显示屏），这个值可能是2或更高。这意味着在高DPI屏幕上，每个CSS像素实际上由多个物理像素组成。
        const outputScale = window.devicePixelRatio || 1;
        return { viewport, outputScale };
    };

    // 使用第1页的高度为所有页面容器预先设置高度，避免首屏滚动条先“跳一下”再稳定
    const InitPageHeights = async () => { 
        if (!pdfDoc.value || totalPages.value === 0) return;

        try {
            const firstPage = await pdfDoc.value.getPage(1);
            const { viewport } = computePageViewport(firstPage);

            // 用 Vue refs 遍历所有页面容器，而不是 querySelectorAll
            for (let i = 1; i <= totalPages.value; i++) {
            const container = pageContainerRefs[i];
            const canvas = pageCanvasRefs[i];
            if (container && canvas) {
                container.style.height = viewport.height + 'px';
                canvas.style.width = viewport.width + 'px';
                canvas.style.height = viewport.height + 'px';
            }
            }
        } catch (e) {
            console.warn('prepareInitialHeights failed', e);
        }
    };

    
    // 渲染单一页面：先算尺寸，再画 canvas，最后再补文本层
    const RenderPage = async (pageNum) => {
        if (renderingPages.value.has(pageNum) || !pdfDoc.value) {
            console.warn('RenderPage: pageNum is rendering or pdfDoc is null');
            return;
        }
        
        console.log(`开始渲染第 ${pageNum} 页`);
        // 标记正在渲染的页码，避免重复渲染
        renderingPages.value.add(pageNum);
        
        try {
            const page = await pdfDoc.value.getPage(pageNum);
            const canvas = pageCanvasRefs[pageNum];
            const textLayer = pageTextLayerRefs[pageNum];
            const pageContainer = pageContainerRefs[pageNum];
            if (!canvas) {
                console.warn(`第 ${pageNum} 页的 canvas 不存在`);
                return;
            }

            const { viewport, outputScale } = computePageViewport(page);
            console.log(viewport);
            // canvas.width和canvas.height是画布的实际像素尺寸，canvas.style.width和canvas.style.height是画布在页面上的显示尺寸
            // 即前者决定了渲染的清晰度，后者决定了画布在页面上的大小。
            // 通过设置canvas.width和canvas.height为viewport的尺寸乘以outputScale，我们确保了在高DPI屏幕上渲染的清晰度。
            canvas.width = Math.floor(viewport.width * outputScale);
            canvas.height = Math.floor(viewport.height * outputScale);
            canvas.style.width = viewport.width + 'px';
            canvas.style.height = viewport.height + 'px';

            // context.setTransform()方法用于设置当前的变换矩阵。通过将outputScale应用于x和y轴，我们确保了在高DPI屏幕上渲染的内容被正确缩放，从而保持清晰度。
            const context = canvas.getContext('2d');
            // context.setTransform()的函数原型是：context.setTransform(a, b, c, d, e, f)，其中a、b、c、d、e、f分别对应变换矩阵的元素。通过将outputScale应用于a和d，我们实现了在x和y轴上的缩放。
            // 即，canvas.width和canvas.height调整了画布大小，而context.setTransform()确保了绘制的内容按照正确的比例缩放，从而在高DPI屏幕上保持清晰。
            context.setTransform(outputScale, 0, 0, outputScale, 0, 0);
            
            // 渲染页面，renderContext包含了canvas的2D渲染上下文和页面的viewport信息，pdf.js会根据这些信息将PDF页面渲染到canvas上。
            const renderContext = {
            canvasContext: context,
            viewport: viewport,
            }; 

            console.log('开始渲染页面内容');
            await page.render(renderContext).promise;

            // 渲染完成后再回写容器高度，防止页面之间重叠
            if (pageContainer) {
                pageContainer.style.height = canvas.style.height;
            }

            // 渲染文本层用于选择
            if (textLayer) {
            try {
                const textContent = await page.getTextContent();
                RenderTextLayer(textContent, viewport, textLayer);
            } catch (e) {
                console.warn('RenderTextLayer failed', e);
            }
            }

            console.log(`第 ${pageNum} 页渲染完成`);
        } catch (err) {
            console.error(`渲染第 ${pageNum} 页失败:`, err);
        } finally {
            renderingPages.value.delete(pageNum);
        }
    };

    
    /**
     * 渲染 PDF 文本层（DOM，用于选择/复制）
     * @param {Object} textContent - page.getTextContent() 的结果
     * @param {PDFPageViewport} viewport - 与 canvas 使用的同一个 viewport
     * @param {HTMLElement} textLayer - .pdf-text-layer 元素
     */
    const RenderTextLayer = (textContent, viewport, textLayer) => {
        if (!textLayer) return;

        textLayer.innerHTML = '';
        textLayer.style.width = viewport.width + 'px';
        textLayer.style.height = viewport.height + 'px';

        const viewportTransform = viewport.transform;

        for (const item of textContent.items) {
            if (!item.str) continue;

            const span = document.createElement('span');
            span.textContent = item.str;

            // PDF → viewport
            const tx = pdfjsLib.Util.transform(
            viewportTransform,
            item.transform
            );

            let [a, b, c, d, e, f] = tx;

            // ⭐ 关键：基线 → 左上角修正
            const ascent = item.height * 0.8;
            f -= ascent;
            // ⭐ 核心：只修正文字方向，不翻整页
            c = -c;
            d = -d;

            span.style.transform = `matrix(${a}, ${b}, ${c}, ${d}, ${e}, ${f})`;
            span.style.transformOrigin = '0 0';

            const font = textContent.styles[item.fontName];
            if (font?.fontFamily) {
            span.style.fontFamily = font.fontFamily;
            }

            span.style.fontSize = '1px';
            span.style.lineHeight = '1';
            span.style.position = 'absolute';
            span.style.whiteSpace = 'pre';
            span.style.userSelect = 'text';
            span.style.webkitUserSelect = 'text';

            textLayer.appendChild(span);
        }
    };

    // 设置交叉观察器：只负责“哪些页进入可视范围后要开始渲染”，不负责决定当前页码
    const SetupIntersectionObserver = () => {
        if (pageObserver) {
            pageObserver.disconnect();
        }

        pageObserver = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
            const pageNum = parseInt(entry.target.dataset.pageNum);
            
            if (entry.isIntersecting && !renderingPages.value.has(pageNum)) {
                currentPages.value.add(pageNum);
                RenderPage(pageNum);
            } else if (!entry.isIntersecting) {
                currentPages.value.delete(pageNum);
            }
            });
            console.log('当前可见页码:', Array.from(currentPages.value));
        }, {
            rootMargin: '200px' // 提前200px开始加载
        });
        
        const pageContainers = pdfCanvas.value?.querySelectorAll('.pdf-page-container');
        pageContainers?.forEach(container => pageObserver.observe(container));
    };

    // 重新渲染可见页面
    const RerenderVisiblePages = async () => {
    if (renderTimeout) clearTimeout(renderTimeout);

    renderTimeout = setTimeout(() => {
        console.log('重新渲染可见页面:', Array.from(currentPages.value));
        currentPages.value.forEach(pageNum => {
        renderingPages.value.delete(pageNum);
        RenderPage(pageNum);
        });
    }, RENDER_THROTTLE);
    };

    onBeforeUnmount(() => {
        if (pageObserver) pageObserver.disconnect();
        if (renderTimeout) clearTimeout(renderTimeout);
    });

    return {
        RegisterPageRefs,
        ClearPages,
        InitPageHeights,
        SetupIntersectionObserver,
        RerenderVisiblePages,
        pageCanvasRefs,
        pageTextLayerRefs,
        pageContainerRefs,
        renderingPages,
        currentPages,
        pageWidth,
    };
    
}
