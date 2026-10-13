const MAX_IMAGE_BYTES = 2 * 1024 * 1024;
const MAX_PRODUCT_IMAGES = 7;
const ALLOWED_IMAGE_TYPES = ['image/jpeg', 'image/png'];

function setImageError(message) {
  const el = document.getElementById('err-image');
  if (el) el.textContent = message || '';
}

function setProductImageError(message) {
  const el = document.getElementById('err-pimages');
  if (el) el.textContent = message || '';
}

// Returns { ok, error } for a candidate image file.
function isAllowedImage(file) {
  if (ALLOWED_IMAGE_TYPES.indexOf(file.type) === -1) {
    return { ok: false, error: 'Images must be JPG, JPEG or PNG' };
  }
  if (file.size > MAX_IMAGE_BYTES) {
    return { ok: false, error: 'Each image must be 2MB or less' };
  }
  return { ok: true, error: '' };
}

// Guards the single category image input. Returns false when rejected so the
// datastar expression skips setting the preview signals.
function acceptCategoryImage(input) {
  const file = input.files[0];
  if (!file) {
    setImageError('Please upload an image');
    return false;
  }
  const result = isAllowedImage(file);
  if (!result.ok) {
    setImageError(result.error);
    input.value = '';
    return false;
  }
  setImageError('');
  return true;
}

function addTag() {
  const input = document.getElementById('tag-input');
  const container = document.getElementById('tags');
  if (!input || !container) return;

  const value = input.value.trim();
  if (!value) return;

  const wrap = document.createElement('div');
  wrap.className = 'flex items-center gap-2 rounded-lg bg-[#EEF2EB] px-2.5 py-1 text-xs text-[#283B28]';

  const hidden = document.createElement('input');
  hidden.type = 'hidden';
  hidden.name = 'tags[]';
  hidden.value = value;

  const span = document.createElement('span');
  span.textContent = value;

  const remove = document.createElement('button');
  remove.type = 'button';
  remove.textContent = '×';
  remove.className = 'font-bold text-[#63685F] hover:text-red-600';
  remove.onclick = function () { wrap.remove(); };

  wrap.appendChild(hidden);
  wrap.appendChild(span);
  wrap.appendChild(remove);
  container.appendChild(wrap);
  input.value = '';
}


window.productImages = [];
window.existingImages = [];

function addProductImages(input) {
  const files = Array.from(input.files);
  const accepted = [];

  for (const file of files) {
    if (window.existingImages.length + window.productImages.length + accepted.length >= MAX_PRODUCT_IMAGES) {
      setProductImageError('You can upload at most ' + MAX_PRODUCT_IMAGES + ' images');
      break;
    }
    const result = isAllowedImage(file);
    if (!result.ok) {
      setProductImageError(result.error);
      break;
    }
    accepted.push(file);
  }

  window.productImages = window.productImages.concat(accepted);

  // Clear the native picker first so re-picking the same file still fires a
  // change event, then let renderProductImages() restore input.files from
  // window.productImages. Clearing value *after* that would wipe the files.
  input.value = '';
  renderProductImages();
}
// Clears the parts of the product form that datastar signals cannot reach:
// tags, chosen images and variant rows. Called after a successful save.
window.resetProductExtras = function () {
  var tags = document.getElementById('tags');
  if (tags) tags.innerHTML = '';

  var tagInput = document.getElementById('tag-input');
  if (tagInput) tagInput.value = '';

  window.productImages = [];
  window.existingImages = [];
  renderProductImages();

  if (typeof window.resetVariants === 'function') window.resetVariants();
};

function appendImagePreview(container, src, isCover, onRemove) {
  const wrap = document.createElement('div');
  wrap.className = 'relative';

  const img = document.createElement('img');
  img.src = src;
  img.className = 'h-12 w-12 rounded-lg object-cover border border-[#E5E8DF]';
  wrap.appendChild(img);

  if (isCover) {
    const badge = document.createElement('span');
    badge.textContent = 'COVER';
    badge.className = 'absolute bottom-0.5 left-0.5 rounded bg-black/70 px-1 py-0.5 text-[8px] font-semibold text-white';
    wrap.appendChild(badge);
  }

  const removeBtn = document.createElement('button');
  removeBtn.type = 'button';
  removeBtn.textContent = '×';
  removeBtn.className = 'absolute -top-1.5 -right-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-red-500 text-[10px] font-bold text-white hover:bg-red-600';
  removeBtn.onclick = onRemove;
  wrap.appendChild(removeBtn);

  container.appendChild(wrap);
  return wrap;
}

// Renders saved images (by URL) followed by newly picked files, then rebuilds
// input.files from the new files only. Saved images stay in existingImages.
function renderProductImages() {
  const container = document.getElementById('image-previews');
  const input = document.getElementById('images-input');
  if (!container || !input) return;

  window.productImages = window.productImages || [];
  window.existingImages = window.existingImages || [];
  container.innerHTML = '';

  window.existingImages.forEach(function (url, i) {
    const wrap = appendImagePreview(container, url, i === 0, function () {
      window.existingImages.splice(i, 1);
      renderProductImages();
    });

    // Tells the server which saved images are still kept on update.
    const hidden = document.createElement('input');
    hidden.type = 'hidden';
    hidden.name = 'existing_images[]';
    hidden.value = url;
    wrap.appendChild(hidden);
  });

  window.productImages.forEach(function (file, i) {
    const isCover = window.existingImages.length === 0 && i === 0;
    appendImagePreview(container, URL.createObjectURL(file), isCover, function () {
      window.productImages.splice(i, 1);
      renderProductImages();
    });
  });

  if (window.existingImages.length + window.productImages.length > 0) setProductImageError('');

  const dt = new DataTransfer();
  window.productImages.forEach(function (file) { dt.items.add(file); });
  input.files = dt.files;
}

// Called by the server after an edit request with the product's saved image URLs.
window.setExistingImages = function (urls) {
  window.existingImages = urls || [];
  window.productImages = [];
  renderProductImages();
};


(function () {
  const container = document.getElementById('variants-container');
  const addButton = document.getElementById('add-variant');
  if (!container || !addButton) return; // not on the products page

  let variantIndex = 0;

  function updateVariantNumbers() {
    const rows = container.querySelectorAll('.variant-row');
    rows.forEach(function (row, index) {
      row.querySelector('.variant-number').textContent = 'Variant ' + (index + 1);
    });
  }

  function updateRemoveButtons() {
    const rows = container.children;
    const buttons = container.querySelectorAll('.remove-variant');
    buttons.forEach(function (button) {
      button.style.display = rows.length > 1 ? '' : 'none';
    });
  }

  function removeVariant(row) {
    row.remove();
    updateVariantNumbers();
    updateRemoveButtons();
  }

  function addVariant() {
    const index = variantIndex++;

    const row = document.createElement('div');
    row.className = 'variant-row rounded-xl border border-[#E5E8DF] bg-[#FBFBF9] p-3';
    row.innerHTML = `
      <div class="flex items-center justify-between">
        <span class="variant-number text-xs font-semibold uppercase tracking-wide text-[#8A9285]"></span>
        <button type="button" class="remove-variant text-xs font-medium text-red-500 hover:text-red-700">Remove</button>
      </div>
      <div class="mt-2 grid grid-cols-4 gap-2">
        <div>
          <label class="mb-1 block text-xs font-medium text-[#63685F]">Color <span class="text-xs text-[#8A9285]">*</span></label>
          <input type="text" name="variants[${index}].color" class="w-full rounded-lg border border-[#DFE5DA] bg-white px-2.5 py-1.5 text-sm text-[#20231F] outline-none focus:border-[#283B28]"/>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-[#63685F]">Size <span class="text-xs text-[#8A9285]">*</span></label>
          <input type="text" name="variants[${index}].size" class="w-full rounded-lg border border-[#DFE5DA] bg-white px-2.5 py-1.5 text-sm text-[#20231F] outline-none focus:border-[#283B28]"/>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-[#63685F]">Stock <span class="text-xs text-[#8A9285]">*</span></label>
          <input type="number" min="0" name="variants[${index}].stock" class="w-full rounded-lg border border-[#DFE5DA] bg-white px-2.5 py-1.5 text-sm text-[#20231F] outline-none focus:border-[#283B28]"/>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-[#63685F]">Price</label>
          <div class="relative">
            <span class="pointer-events-none absolute inset-y-0 left-2.5 flex items-center text-xs text-[#8A9285]">Rs.</span>
            <input type="text" name="variants[${index}].price" placeholder="Base" class="w-full rounded-lg border border-[#DFE5DA] bg-white py-1.5 pl-8 pr-2.5 text-sm text-[#20231F] outline-none focus:border-[#283B28]"/>
          </div>
        </div>
      </div>
    `;

    container.appendChild(row);
    updateVariantNumbers();
    updateRemoveButtons();
  }

  addButton.addEventListener('click', function () { addVariant(); });

  container.addEventListener('click', function (event) {
    const button = event.target.closest('.remove-variant');
    if (!button) return;
    const row = button.closest('.variant-row');
    if (row) removeVariant(row);
  });

  window.resetVariants = function () {
    variantIndex = 0;
    container.innerHTML = '';
    addVariant();
  };

  // The edit response replaces the rows server-side; align the counter so the
  // next "+ Add variant" continues numbering instead of colliding.
  window.syncVariantIndex = function () {
    variantIndex = container.querySelectorAll('.variant-row').length;
    updateRemoveButtons();
  };

  addVariant();
})();