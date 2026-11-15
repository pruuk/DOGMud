// MP3Player: the web client's music player (one track at a time) and sound
// effect player (allowMultiple). The web client makes no sound until the
// player turns sound on (owner, 2026-11-15): starting an audio element at
// all, even at volume 0, can take a phone's audio away from the player's own
// music. So a player starts nothing while it is disabled, and nothing at
// volume 0. A music player remembers the track it was last asked for and
// starts it when sound is turned on or Music is raised from 0; sound effects
// are one-offs and are never replayed.
class MP3Player {
  constructor(allowMultiple = false) {
    this.audioCache = new Map();
    this.currentUrl = null;
    this.loop = true;
    this.allowMultiple = allowMultiple;
    this.activeAudios = new Set();
    this.enabled = false;   // off until the player turns sound on
    this.wanted = null;     // music only: { url, loop } the server last asked for
  }

  // Turn this player on or off. level: the volume to start the wanted track
  // at when turning on (music only).
  setEnabled(on, level = 1.0) {
    this.enabled = !!on;
    if (!this.enabled) {
      this._halt();
      return;
    }
    if (!this.allowMultiple && this.wanted && level > 0) {
      this._start(this.wanted.url, this.wanted.loop, level);
    }
  }

  play(url, loop = true, volume = 1.0) {
    // Ensure volume doesn't exceed 1.0
    if (volume > 1.0) {
      volume = 1.0;
    }
    if (!this.allowMultiple) {
      this.wanted = { url: url, loop: loop };
    }
    if (!this.enabled || !(volume > 0)) {
      // Off, or at 0: start nothing. A track already playing stops, so a
      // new area's music never leaves the old one looping.
      if (!this.allowMultiple) {
        this._halt();
      }
      return;
    }
    this._start(url, loop, volume);
  }

  _start(url, loop, volume) {
    if (!this.allowMultiple) {
      this._halt();
    }

    let audio;
    if (this.audioCache.has(url)) {
      audio = this.audioCache.get(url);
    } else {
      audio = new Audio();
      audio.src = url;
      audio.preload = "auto";
      this.audioCache.set(url, audio);
    }

    audio.loop = loop;
    audio.volume = volume;
    audio.play().catch((e) => console.error("Playback failed:", e));

    // Keep track of currently playing audio
    this.activeAudios.add(audio);

    this.currentUrl = url;
  }

  // Stop what is playing but remember the wanted track.
  _halt() {
    this.activeAudios.forEach(audio => {
      audio.pause();
      audio.currentTime = 0;
    });
    this.activeAudios.clear();
  }

  pause(url) {
    if (this.audioCache.has(url)) {
      this.audioCache.get(url).pause();
    }
  }

  stop(url) {
    if (this.audioCache.has(url)) {
      let audio = this.audioCache.get(url);
      audio.pause();
      audio.currentTime = 0;
      this.activeAudios.delete(audio);
    }
  }

  // Stop everything and forget the wanted track: the server turned music off,
  // or the player left the game.
  stopAll() {
    this._halt();
    this.wanted = null;
  }

  setVolume(url, level) {
    // Set volume for a specific URL (if needed)
    if (level < 0 || level > 1) {
      console.error("Volume must be between 0 and 1");
      return;
    }
    if (this.audioCache.has(url)) {
      this.audioCache.get(url).volume = level;
    }
  }

  // The Music slider moved. At 0 the track stops instead of playing silently;
  // raised from 0, the wanted track starts again.
  setGlobalVolume(level) {
    if (level < 0 || level > 1) {
      console.error("Volume must be between 0 and 1");
      return;
    }
    if (level === 0) {
      this._halt();
      return;
    }
    if (this.activeAudios.size > 0) {
      this.activeAudios.forEach(audio => {
        audio.volume = level;
      });
      return;
    }
    if (this.enabled && !this.allowMultiple && this.wanted) {
      this._start(this.wanted.url, this.wanted.loop, level);
    }
  }

  setLoop(url, loop) {
    if (this.audioCache.has(url)) {
      this.audioCache.get(url).loop = loop;
    }
  }

  isPlaying(url) {
    return this.audioCache.has(url) && !this.audioCache.get(url).paused;
  }

  getCurrentTime(url) {
    return this.audioCache.has(url) ? this.audioCache.get(url).currentTime : 0;
  }

  getDuration(url) {
    return this.audioCache.has(url) ? this.audioCache.get(url).duration : 0;
  }
}
