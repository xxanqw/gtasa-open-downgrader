import requests
import re
import os
import zipfile
import io

def safe_request(method, url, **kwargs):
    try:
        return method(url, verify=True, **kwargs)
    except requests.exceptions.SSLError:
        import urllib3
        urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
        return method(url, verify=False, **kwargs)

def resolve_icloud_link(url):
    match = re.search(r'/iclouddrive/([^#?]+)', url)
    if not match:
        return None
    
    short_id = match.group(1)
    
    resolve_url = 'https://ckdatabasews.icloud.com/database/1/com.apple.cloudkit/production/public/records/resolve'
    payload = {"shortGUIDs": [{"value": short_id}]}
    
    try:
        response = safe_request(requests.post, resolve_url, json=payload, timeout=30)
        response.raise_for_status()
        data = response.json()

        results = data.get('results', [])
        if not results:
            return None
            
        root_record = results[0].get('rootRecord', {})
        fields = root_record.get('fields', {})
        file_content = fields.get('fileContent', {})
        value = file_content.get('value', {})
        
        return value.get('downloadURL')
    except Exception as e:
        print(f"Error resolving iCloud link: {e}")
        return None

def download_and_extract_patches(url, target_dir, progress_callback=None):
    download_url = resolve_icloud_link(url)
    if not download_url:
        return False
        
    try:
        response = safe_request(requests.get, download_url, stream=True, timeout=60)
        response.raise_for_status()
        
        total_size = int(response.headers.get('content-length', 0))
        downloaded = 0
        buffer = io.BytesIO()
        
        for chunk in response.iter_content(chunk_size=8192):
            if chunk:
                buffer.write(chunk)
                downloaded += len(chunk)
                if progress_callback:
                    progress_callback(downloaded, total_size)
        
        with zipfile.ZipFile(buffer) as z:
            os.makedirs(target_dir, exist_ok=True)
            z.extractall(target_dir)
            
        return True
    except Exception as e:
        print(f"Failed to download/extract: {e}")
        return False